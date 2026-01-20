```
# First run (creates cluster)
make test-e2e-local SKIP_RESOURCE_CLEANUP=true
# Subsequent runs (reuses cluster)
make test-e2e-local USE_EXISTING_CLUSTER=true SKIP_RESOURCE_CLEANUP=true
```
