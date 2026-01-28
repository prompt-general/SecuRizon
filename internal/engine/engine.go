package engine

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/securazion/remediation-engine/internal/executor"
    "github.com/securazion/remediation-engine/internal/playbook"
    "github.com/securazion/remediation-engine/internal/store"
    "github.com/securazion/remediation-engine/internal/workflow"
)

type RemediationEngine struct {
    executor        *executor.Executor
    approvalManager *workflow.ApprovalManager
    playbookManager *playbook.Manager
    store           store.Store
    workQueue       chan RemediationWorkItem
    workers         int
    mu              sync.RWMutex
    metrics         *RemediationMetrics
}

type RemediationWorkItem struct {
    ID         string                 `json:"id"`
    FindingID  string                 `json:"finding_id"`
    PlaybookID string                 `json:"playbook_id"`
    Parameters map[string]interface{} `json:"parameters"`
    Requestor  string                 `json:"requestor"`
    Priority   int                    `json:"priority"`
    CreatedAt  time.Time              `json:"created_at"`
}

type RemediationStatus string

const (
    StatusPending    RemediationStatus = "pending"
    StatusApproved   RemediationStatus = "approved"
    StatusExecuting  RemediationStatus = "executing"
    StatusCompleted  RemediationStatus = "completed"
    StatusFailed     RemediationStatus = "failed"
    StatusRolledBack RemediationStatus = "rolled_back"
    StatusCancelled  RemediationStatus = "cancelled"
)

func NewRemediationEngine(exec *executor.Executor, approval *workflow.ApprovalManager, 
    playbookMgr *playbook.Manager, store store.Store) *RemediationEngine {
    
    return &RemediationEngine{
        executor:        exec,
        approvalManager: approval,
        playbookManager: playbookMgr,
        store:           store,
        workQueue:       make(chan RemediationWorkItem, 1000),
        workers:         5,
        metrics:         NewRemediationMetrics(),
    }
}

func (re *RemediationEngine) Start(ctx context.Context) {
    // Start worker pool
    for i := 0; i < re.workers; i++ {
        go re.worker(ctx, i)
    }

    // Subscribe to Kafka topic for remediation requests
    go re.consumeRemediationRequests(ctx)
    
    // Start periodic cleanup of old jobs
    go re.cleanupOldJobs(ctx)
    
    log.Printf("Remediation engine started with %d workers", re.workers)
}

func (re *RemediationEngine) worker(ctx context.Context, id int) {
    log.Printf("Remediation worker %d started", id)
    
    for {
        select {
        case <-ctx.Done():
            return
        case work := <-re.workQueue:
            re.processWorkItem(ctx, work)
        }
    }
}

func (re *RemediationEngine) processWorkItem(ctx context.Context, work RemediationWorkItem) {
    startTime := time.Now()
    re.metrics.RemediationStarted(work.PlaybookID, work.Requestor)
    
    // Update status to executing
    re.store.UpdateRemediationStatus(ctx, work.ID, string(StatusExecuting), nil)
    
    // Fetch the playbook
    pb, err := re.playbookManager.GetPlaybook(work.PlaybookID)
    if err != nil {
        log.Printf("Failed to get playbook %s: %v", work.PlaybookID, err)
        re.metrics.RemediationFailed(work.PlaybookID, "playbook_not_found")
        return
    }
    
    // Execute the playbook
    result, err := re.executePlaybook(ctx, pb, work)
    if err != nil {
        log.Printf("Playbook execution failed: %v", err)
        re.metrics.RemediationFailed(work.PlaybookID, "execution_failed")
        
        // Update status to failed
        re.store.UpdateRemediationStatus(ctx, work.ID, string(StatusFailed), map[string]interface{}{
            "error": err.Error(),
        })
        
        // Attempt rollback if configured
        if pb.RollbackEnabled {
            re.executeRollback(ctx, pb, work, result)
        }
        
        return
    }
    
    // Update status to completed
    re.store.UpdateRemediationStatus(ctx, work.ID, string(StatusCompleted), map[string]interface{}{
        "outputs":    result.Outputs,
        "duration":   time.Since(startTime).Seconds(),
        "completed_at": time.Now(),
    })
    
    // Emit event
    re.emitRemediationEvent(work, "completed", result)
    
    // Update metrics
    duration := time.Since(startTime)
    re.metrics.RemediationCompleted(work.PlaybookID, duration)
    
    log.Printf("Remediation %s completed in %v", work.ID, duration)
}

func (re *RemediationEngine) executePlaybook(ctx context.Context, pb playbook.Playbook, 
    work RemediationWorkItem) (*playbook.ExecutionResult, error) {
    
    // Pre-flight checks
    if err := re.preFlightChecks(ctx, pb, work); err != nil {
        return nil, fmt.Errorf("pre-flight check failed: %v", err)
    }
    
    // Execute steps
    var outputs []map[string]interface{}
    var executionLogs []playbook.ExecutionLog
    
    for i, step := range pb.Steps {
        log.Printf("Executing step %d: %s", i+1, step.Name)
        
        // Check if step should be skipped
        if step.Condition != "" {
            shouldExecute, err := re.evaluateCondition(step.Condition, work.Parameters)
            if err != nil || !shouldExecute {
                log.Printf("Skipping step %d due to condition", i+1)
                executionLogs = append(executionLogs, playbook.ExecutionLog{
                    Step:    i,
                    Status:  "skipped",
                    Message: "Condition not met",
                })
                continue
            }
        }
        
        // Execute step
        start := time.Now()
        output, err := re.executor.ExecuteStep(ctx, step, work.Parameters)
        duration := time.Since(start)
        
        logEntry := playbook.ExecutionLog{
            Step:     i,
            Action:   step.Action,
            Duration: duration.Seconds(),
            Started:  start,
            Ended:    time.Now(),
        }
        
        if err != nil {
            logEntry.Status = "failed"
            logEntry.Error = err.Error()
            executionLogs = append(executionLogs, logEntry)
            
            return &playbook.ExecutionResult{
                Success:      false,
                FailedStep:   i,
                Outputs:      outputs,
                Logs:         executionLogs,
                RollbackData: re.collectRollbackData(outputs),
            }, fmt.Errorf("step %d failed: %v", i+1, err)
        }
        
        logEntry.Status = "completed"
        logEntry.Output = output
        executionLogs = append(executionLogs, logEntry)
        outputs = append(outputs, output)
        
        // Store checkpoint for potential rollback
        if pb.RollbackEnabled {
            re.storeCheckpoint(ctx, work.ID, i, outputs)
        }
    }
    
    return &playbook.ExecutionResult{
        Success: true,
        Outputs: outputs,
        Logs:    executionLogs,
    }, nil
}

func (re *RemediationEngine) executeRollback(ctx context.Context, pb playbook.Playbook, 
    work RemediationWorkItem, result *playbook.ExecutionResult) {
    
    log.Printf("Executing rollback for remediation %s", work.ID)
    
    // Load rollback steps
    rollbackSteps := pb.RollbackSteps
    if len(rollbackSteps) == 0 {
        // Generate reverse steps automatically
        rollbackSteps = re.generateRollbackSteps(pb, result.FailedStep)
    }
    
    // Execute rollback in reverse order
    for i := len(rollbackSteps) - 1; i >= 0; i-- {
        step := rollbackSteps[i]
        log.Printf("Executing rollback step %d: %s", i+1, step.Name)
        
        _, err := re.executor.ExecuteStep(ctx, step, work.Parameters)
        if err != nil {
            log.Printf("Rollback step %d failed: %v", i+1, err)
            // Continue with other rollback steps
        }
    }
    
    // Update status
    re.store.UpdateRemediationStatus(ctx, work.ID, string(StatusRolledBack), map[string]interface{}{
        "failed_step": result.FailedStep,
        "rollback_at": time.Now(),
    })
    
    re.metrics.RemediationRolledBack(pb.ID)
}

func (re *RemediationEngine) preFlightChecks(ctx context.Context, pb playbook.Playbook, 
    work RemediationWorkItem) error {
    
    // Check if playbook is enabled
    if !pb.Enabled {
        return fmt.Errorf("playbook is disabled")
    }
    
    // Check rate limits
    if !re.checkRateLimit(work.Requestor, pb.ID) {
        return fmt.Errorf("rate limit exceeded for playbook %s", pb.ID)
    }
    
    // Validate parameters
    for _, param := range pb.Parameters {
        if param.Required {
            if _, exists := work.Parameters[param.Name]; !exists {
                return fmt.Errorf("required parameter missing: %s", param.Name)
            }
        }
    }
    
    // Check for concurrent executions on same resource
    concurrent, err := re.checkConcurrentExecutions(ctx, work)
    if err != nil {
        return fmt.Errorf("failed to check concurrent executions: %v", err)
    }
    if concurrent > 0 {
        return fmt.Errorf("concurrent execution detected on same resource")
    }
    
    // Dry run if requested
    if pb.DryRun {
        log.Printf("Dry run enabled for playbook %s", pb.ID)
        re.performDryRun(ctx, pb, work)
    }
    
    return nil
}

func (re *RemediationEngine) RequestRemediation(ctx context.Context, findingID string, 
    playbookID string, parameters map[string]interface{}, requestor string) (string, error) {
    
    // Generate remediation ID
    remediationID := generateUUID()
    
    // Check if approval is required
    pb, err := re.playbookManager.GetPlaybook(playbookID)
    if err != nil {
        return "", fmt.Errorf("playbook not found: %v", err)
    }
    
    workItem := RemediationWorkItem{
        ID:         remediationID,
        FindingID:  findingID,
        PlaybookID: playbookID,
        Parameters: parameters,
        Requestor:  requestor,
        Priority:   pb.Priority,
        CreatedAt:  time.Now(),
    }
    
    // Store in database
    if err := re.store.CreateRemediation(ctx, workItem); err != nil {
        return "", fmt.Errorf("failed to create remediation: %v", err)
    }
    
    // If approval is required, create approval request
    if pb.ApprovalRequired {
        approvalID, err := re.approvalManager.CreateApprovalRequest(ctx, workItem)
        if err != nil {
            return "", fmt.Errorf("failed to create approval request: %v", err)
        }
        
        // Update remediation with approval ID
        re.store.UpdateRemediationApproval(ctx, remediationID, approvalID)
        
        return remediationID, nil
    }
    
    // If no approval required, queue for execution
    re.workQueue <- workItem
    
    return remediationID, nil
}
