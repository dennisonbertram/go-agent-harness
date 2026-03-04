# Deployment Runbook

## Goal

Deploy MVP safely with practical controls; do not over-engineer for enterprise scale.

## Pre-Deployment Checklist

- [ ] Feature plan completed and checked off.
- [ ] Tests pass in CI/local.
- [ ] Security-sensitive changes reviewed.
- [ ] Required environment variables documented.
- [ ] Rollback steps prepared.

## Deployment Steps

1. Merge tested branch into `main`.
2. Trigger deployment pipeline.
3. Run smoke tests on deployed environment.
4. Validate critical user paths.
5. Log deployment result in engineering log.

## Post-Deployment

- [ ] Monitor error rates and key metrics.
- [ ] Record anomalies in observational log.
- [ ] Create GitHub issues for any discovered defects.
