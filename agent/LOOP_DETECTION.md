# Loop Detection

No loops detected yet.

## Rule

If a task has three blocked attempts with little or no progress, split it into smaller subtasks, record the obstacle here, and ask the user to review.

## Suggested JSONL query

```bash
jq -r '[.task_id,.status] | @tsv' agent/ATTEMPT_LOG.jsonl
```
