---
name: deploy
description: Deploy the selected service
argument-hint: '[service]'
plugxfer-origin: command
plugxfer-source-model: sonnet
---

## Command Template


Use this runbook:

<!-- plugxfer: materialized @runbook.txt -->

### Imported `runbook.txt`

```
Validate the service and deploy only after approval.
```

Then run:

${PLUGIN_ROOT}/bin/deploy-helper $ARGUMENTS

The literal !`unsafe example` must become inert in Codex.
