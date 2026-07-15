#!/usr/bin/env python3
from pathlib import Path
import sys

required = [
    'README.md',
    'LICENSE',
    'PLAN.md',
    'AGENTS.md',
    'ROADMAP.md',
    'SYSTEM_ARCHITECTURE.md',
    'API_SPEC.md',
    'CODING_STANDARDS.md',
    'TESTING_POLICY.md',
    'ENVIRONMENT_SETUP.md',
    'CONTEXT_MAP.md',
    'PROMPT.md',
    'FEATURE_MATRIX.md',
    'UI_DESIGN.md',
    'PUBLISHING_POLICY.md',
    'VERSIONING_AND_SYNC_POLICY.md',
    'WORKSPACE_MAINTENANCE.md',
    'SCAFFOLD_REVIEW_REPORT.md',
    'SCAFFOLD_STEP3_REPORT.md',
    'SCAFFOLD_STEP4_REPORT.md',
    'SCAFFOLD_STEP5_REPORT.md',
    'skills/agent-handoff/SKILL.md',
    'skills/release-packaging/SKILL.md',
    'prompts/start_agent_from_handoff.md',
    'SCAFFOLD_STEP6_REPORT.md',
    'CODING_CLIENT_HANDOFF.md',
    'CLAUDE.md',
    'NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md',
    'SEARCH_QUERY_LANGUAGE.md',
    'RECOLL_INTEGRATION.md',
    'DOCS_SITE.md',
    'MVP_TASK1_REPORT.md',
    'MVP_TASK2_REPORT.md',
    'MVP_TASK4_REPORT.md',
    'MVP_TASK5_REPORT.md',
    'MVP_TASK6_REPORT.md',
    'MVP_TASK7_REPORT.md',
    'MVP_TASK8_REPORT.md',
    'MVP_TASK9_REPORT.md',
    'PACKAGING.md',
    'SECURITY_REVIEW.md',
    'RELEASE_CHECKLIST.md',
    'MVP_RELEASE_REPORT.md',
    'MVP_TASK10_REPORT.md',
    'scripts/mvp_smoke.sh',
    'scripts/run_performance_smoke.sh',
    'scripts/package_release.sh',
    'scripts/check_release_zip.py',
    'DATABASE_SCHEMA.md',
    'SCAFFOLD_CREATION_PLAN.md',
    'agent/PLAN_STATUS.md',
    'agent/ATTEMPT_LOG.jsonl',
    'agent/MODEL_LOG.jsonl',
    'api/openapi.yaml',
    'config/config.example.yaml',
    'cmd/notriosd/main.go',
    'cmd/notriosctl/main.go',
    'web/package.json',
    'web/package-lock.json',
    'web/README.md',
    'web/src/api.ts',
    'web/src/App.tsx',
    'web/src/vite-env.d.ts',
]

missing = [p for p in required if not Path(p).exists()]
if missing:
    print('Missing required files:')
    for p in missing:
        print(f'  - {p}')
    sys.exit(1)

print(f'All {len(required)} required scaffold files exist.')
