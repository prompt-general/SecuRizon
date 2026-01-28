terraform {
  required_version = ">= 1.3.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.0"
    }
  }
  
  backend "s3" {
    bucket         = "securazion-terraform-state"
    key            = "global/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"
  }
}

# Multi-region configuration
module "primary_region" {
  source = "./modules/region"
  
  region          = "us-east-1"
  environment     = var.environment
  vpc_cidr        = "10.0.0.0/16"
  cluster_version = "1.27"
  
  # Node groups
  node_groups = {
    general = {
      instance_types = ["m5.large", "m5a.large"]
      min_size       = 3
      max_size       = 10
      desired_size   = 5
    }
    memory = {
      instance_types = ["r5.large", "r5a.large"]
      min_size       = 2
      max_size       = 6
      desired_size   = 3
    }
  }
}

module "secondary_region" {
  source = "./modules/region"
  
  region          = "eu-west-1"
  environment     = var.environment
  vpc_cidr        = "10.1.0.0/16"
  cluster_version = "1.27"
  
  # Node groups
  node_groups = {
    general = {
      instance_types = ["m5.large", "m5a.large"]
      min_size       = 2
      max_size       = 6
      desired_size   = 3
    }
  }
}

# Global resources
module "global" {
  source = "./modules/global"
  
  primary_region   = "us-east-1"
  secondary_region = "eu-west-1"
  
  # Route53 configuration
  domain_name = var.domain_name
  subdomains = {
    api     = "api.${var.domain_name}"
    app     = "app.${var.domain_name}"
    metrics = "metrics.${var.domain_name}"
  }
}

# Database replication
resource "aws_rds_global_cluster" "neo4j" {
  global_cluster_identifier = "securazion-neo4j-global"
  engine                    = "aurora-postgresql"
  engine_version            = "13.7"
  database_name             = "securazion"
  storage_encrypted         = true
}

resource "aws_rds_cluster" "primary" {
  cluster_identifier      = "securazion-neo4j-primary"
  engine                 = aws_rds_global_cluster.neo4j.engine
  engine_version         = aws_rds_global_cluster.neo4j.engine_version
  global_cluster_identifier = aws_rds_global_cluster.neo4j.id
  database_name          = "securazion"
  master_username        = var.db_username
  master_password        = random_password.db_password.result
  backup_retention_period = 35
  preferred_backup_window = "07:00-09:00"
  skip_final_snapshot    = false
  final_snapshot_identifier = "securazion-neo4j-final-snapshot"
  deletion_protection    = true
  
  vpc_security_group_ids = [module.primary_region.db_security_group_id]
  db_subnet_group_name   = module.primary_region.db_subnet_group_name
  
  enabled_cloudwatch_logs_exports = ["postgresql"]
  
  lifecycle {
    ignore_changes = [master_password]
  }
}

resource "aws_rds_cluster_instance" "primary" {
  count              = 2
  identifier         = "securazion-neo4j-primary-${count.index}"
  cluster_identifier = aws_rds_cluster.primary.id
  instance_class     = "db.r5.large"
  engine            = aws_rds_cluster.primary.engine
  engine_version    = aws_rds_cluster.primary.engine_version
  
  performance_insights_enabled = true
  monitoring_interval         = 60
  monitoring_role_arn        = aws_iam_role.rds_monitoring.arn
}

resource "aws_rds_cluster" "secondary" {
  provider = aws.secondary
  
  cluster_identifier      = "securazion-neo4j-secondary"
  engine                 = aws_rds_global_cluster.neo4j.engine
  engine_version         = aws_rds_global_cluster.neo4j.engine_version
  global_cluster_identifier = aws_rds_global_cluster.neo4j.id
  source_region          = "us-east-1"
  
  vpc_security_group_ids = [module.secondary_region.db_security_group_id]
  db_subnet_group_name   = module.secondary_region.db_subnet_group_name
  
  depends_on = [aws_rds_cluster.primary]
}

# MSK (Kafka) multi-region
resource "aws_msk_cluster" "primary" {
  cluster_name           = "securazion-kafka-primary"
  kafka_version          = "3.3.1"
  number_of_broker_nodes = 3
  
  broker_node_group_info {
    instance_type   = "kafka.m5.large"
    ebs_volume_size = 1000
    client_subnets  = module.primary_region.private_subnets
    security_groups = [module.primary_region.kafka_security_group_id]
  }
  
  encryption_info {
    encryption_at_rest_kms_key_arn = aws_kms_key.kafka.arn
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }
  
  configuration_info {
    arn      = aws_msk_configuration.config.arn
    revision = aws_msk_configuration.config.latest_revision
  }
  
  open_monitoring {
    prometheus {
      jmx_exporter {
        enabled_in_broker = true
      }
      node_exporter {
        enabled_in_broker = true
      }
    }
  }
  
  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = aws_cloudwatch_log_group.kafka.name
      }
    }
  }
}

# Global Load Balancer
resource "aws_route53_record" "api" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = "api.${var.domain_name}"
  type    = "CNAME"
  ttl     = 300
  records = [aws_globalaccelerator_accelerator.api.dns_name]
}

resource "aws_globalaccelerator_accelerator" "api" {
  name            = "securazion-api"
  ip_address_type = "IPV4"
  enabled         = true
  
  attributes {
    flow_logs_enabled   = true
    flow_logs_s3_bucket = aws_s3_bucket.global_accelerator_logs.bucket
    flow_logs_s3_prefix = "logs/"
  }
}

resource "aws_globalaccelerator_listener" "api" {
  accelerator_arn = aws_globalaccelerator_accelerator.api.id
  client_affinity = "SOURCE_IP"
  protocol        = "TCP"
  
  port_range {
    from_port = 443
    to_port   = 443
  }
}

resource "aws_globalaccelerator_endpoint_group" "primary" {
  listener_arn = aws_globalaccelerator_listener.api.id
  
  endpoint_configuration {
    endpoint_id = module.primary_region.nlb_arn
    weight      = 100
  }
  
  health_check_port = 80
  health_check_protocol = "TCP"
  
  traffic_dial_percentage = 100
}

resource "aws_globalaccelerator_endpoint_group" "secondary" {
  listener_arn = aws_globalaccelerator_listener.api.id
  
  endpoint_configuration {
    endpoint_id = module.secondary_region.nlb_arn
    weight      = 0
  }
  
  health_check_port = 80
  health_check_protocol = "TCP"
  
  traffic_dial_percentage = 0
  
  # Failover configuration
  endpoint_configuration {
    endpoint_id = module.primary_region.nlb_arn
    weight      = 0
  }
}
