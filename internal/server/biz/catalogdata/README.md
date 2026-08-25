Embedded catalog snapshots for the AxonHub binary.

`providers.json` is the filtered developer catalog fallback.
`models.json` is extra/canonical model metadata merged after an upstream fetch.

`scripts/sync/sync-model-developers.js` refreshes both files when the weekly sync runs.
