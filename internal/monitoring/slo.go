package monitoring

import (
    "context"
    "fmt"
    "time"
)

type SLODefinition struct {
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Objective   float64      `json:"objective"` // e.g., 99.9 for 99.9%
    Period      string       `json:"period"`    // "28d", "7d", "24h"
    Metrics     []SLOMetric  `json:"metrics"`
}

type SLOMetric struct {
    Name      string  `json:"name"`
    Query     string  `json:"query"`
    Threshold float64 `json:"threshold"`
    Operator  string  `json:"operator"` // "gt", "lt", "eq"
}

type SLOEvaluation struct {
    Name      string            `json:"name"`
    Timestamp time.Time         `json:"timestamp"`
    Period    string            `json:"period"`
    Objective float64           `json:"objective"`
    Score     float64           `json:"score"`
    Met       bool              `json:"met"`
    Metrics   []MetricEvaluation `json:"metrics"`
}

type MetricEvaluation struct {
    Name      string  `json:"name"`
    Value     float64 `json:"value"`
    Threshold float64 `json:"threshold"`
    Operator  string  `json:"operator"`
    Met       bool    `json:"met"`
}

type MetricsClient interface {
    Query(ctx context.Context, query string) (PrometheusResult, error)
}

type PrometheusResult struct {
    Data struct {
        Result []struct {
            Value interface{} `json:"value"`
        } `json:"result"`
    } `json:"data"`
}

type SLOTracker struct {
    sloDefinitions map[string]SLODefinition
    metricsClient  MetricsClient
}

func NewSLOTracker(client MetricsClient) *SLOTracker {
    return &SLOTracker{
        sloDefinitions: make(map[string]SLODefinition),
        metricsClient:  client,
    }
}

func (st *SLOTracker) AddSLO(slo SLODefinition) {
    st.sloDefinitions[slo.Name] = slo
}

func (st *SLOTracker) EvaluateSLO(ctx context.Context, sloName string) (*SLOEvaluation, error) {
    slo, exists := st.sloDefinitions[sloName]
    if !exists {
        return nil, fmt.Errorf("SLO %s not found", sloName)
    }

    evaluation := &SLOEvaluation{
        Name:      slo.Name,
        Timestamp: time.Now(),
        Period:    slo.Period,
        Objective: slo.Objective,
    }

    var totalScore float64
    var metObjectives int

    for _, metric := range slo.Metrics {
        metricValue, err := st.evaluateMetric(ctx, metric)
        if err != nil {
            return nil, fmt.Errorf("failed to evaluate metric %s: %v", metric.Name, err)
        }

        metricEval := MetricEvaluation{
            Name:      metric.Name,
            Value:     metricValue,
            Threshold: metric.Threshold,
            Operator:  metric.Operator,
            Met:       st.checkMetric(metricValue, metric.Threshold, metric.Operator),
        }

        evaluation.Metrics = append(evaluation.Metrics, metricEval)

        if metricEval.Met {
            metObjectives++
        }
    }

    if len(evaluation.Metrics) > 0 {
        evaluation.Score = (float64(metObjectives) / float64(len(evaluation.Metrics))) * 100
        evaluation.Met = evaluation.Score >= slo.Objective
    }

    return evaluation, nil
}

func (st *SLOTracker) evaluateMetric(ctx context.Context, metric SLOMetric) (float64, error) {
    result, err := st.metricsClient.Query(ctx, metric.Query)
    if err != nil {
        return 0, err
    }
    if len(result.Data.Result) == 0 {
        return 0, fmt.Errorf("no data returned for metric")
    }
    // Assuming the value is a string that can be parsed to float64
    switch v := result.Data.Result[0].Value.(type) {
    case string:
        var f float64
        _, err := fmt.Sscanf(v, "%f", &f)
        if err != nil {
            return 0, fmt.Errorf("invalid metric value type")
        }
        return f, nil
    case float64:
        return v, nil
    default:
        return 0, fmt.Errorf("invalid metric value type")
    }
}

func (st *SLOTracker) checkMetric(value, threshold float64, operator string) bool {
    switch operator {
    case "gt":
        return value > threshold
    case "lt":
        return value < threshold
    case "eq":
        return value == threshold
    default:
        return false
    }
}

// Example SLO definitions for SecuRizon
var DefaultSLOs = []SLODefinition{
    {
        Name:        "event-processing-latency",
        Description: "Event processing should complete within 30 seconds",
        Objective:   99.9,
        Period:      "7d",
        Metrics: []SLOMetric{{
            Name:      "p95_event_processing_latency",
            Query:     "histogram_quantile(0.95, rate(securazion_event_processing_duration_seconds_bucket[5m]))",
            Threshold: 30,
            Operator:  "lt",
        }},
    },
    {
        Name:        "remediation-success-rate",
        Description: "Remediation actions should succeed at least 95% of the time",
        Objective:   99.5,
        Period:      "28d",
        Metrics: []SLOMetric{{
            Name:      "remediation_success_rate",
            Query:     "rate(securazion_remediation_success_total[1h]) / rate(securazion_remediation_total[1h]) * 100",
            Threshold: 95,
            Operator:  "gt",
        }},
    },
    {
        Name:        "real-time-detection",
        Description: "Security findings should be detected within 1 minute",
        Objective:   99.0,
        Period:      "7d",
        Metrics: []SLOMetric{{
            Name:      "detection_latency_p95",
            Query:     "histogram_quantile(0.95, rate(securazion_finding_detection_latency_seconds_bucket[5m]))",
            Threshold: 60,
            Operator:  "lt",
        }},
    },
}
