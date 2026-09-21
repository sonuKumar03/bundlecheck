# BundleCheck skill evaluation

Run these cases in fresh agent sessions so earlier prompts cannot prime activation or workflow choices.

## Trigger protocol

Run every prompt in `triggers.yaml` three times, each in a fresh session. Record whether the BundleCheck skill activated and compare it with `should_trigger`.

| Case | Run | Expected | Observed | Pass |
| --- | --- | --- | --- | --- |
| example | 1 | activate |  |  |

Acceptance requires at least 90% activation across positive runs and at least 90% rejection across negative runs.

## Workflow protocol

Run every prompt in `workflows.yaml` once in a fresh session. Record each required stage that appeared and any forbidden claim.

| Case | Required stages observed | Forbidden claims observed | Pass |
| --- | --- | --- | --- |
| example |  |  |  |

Acceptance requires 100% inclusion of every case's `required_stages` and zero `forbidden_claims`.

Keep results in a temporary table outside the fixtures. When a case fails, make the smallest description or instruction change that addresses it, then rerun the complete trigger and workflow corpus.
