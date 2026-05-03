# Branch 03: S3 backend

**Goal:** Move Terraform state to S3, add state locking (DynamoDB). Enables CI and other machines to run `terraform plan` against real state. Assumes branch 2 (API Gateway) is done. CI comes in a separate branch.

## What we're adding

- **S3 bucket** - Store Terraform state so CI and other machines can run `terraform plan` against real state
- **DynamoDB table** - State locking (prevents concurrent apply from corrupting state)
- **Backend config** - `backend "s3"` in Terraform
- **State migration** - Copy local state to S3 (one-time)

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| State location | S3 | Shared, durable; CI can init and plan against real state. |
| Locking | DynamoDB | Prevents race conditions when two applies run. |
| Bucket creation | Manual or bootstrap Terraform | Avoid chicken-egg: main Terraform shouldn't manage the bucket it uses for its own state. |

## Prerequisites (before starting)

- Phase 0 steps 1-10 complete (Lambda, API Gateway, local state working)
- AWS CLI configured

## Steps (in order)

1. Create S3 bucket (versioning on). Create DynamoDB table with `LockID` (string) as partition key.
2. Add `backend "s3"` block to `terraform` in main.tf.
3. Run `terraform init`, migrate state when prompted (answer yes).
4. Run `terraform plan` (expect "No changes").
5. Optional: remove local `terraform.tfstate` and `terraform.tfstate.backup`.

## Files changed/added

- `terraform/main.tf` - Add backend block to terraform block

## Next branch

**04-github-actions** - CI workflow (build, test, lint, terraform plan). Or **04-domain-types** - Stripe event and internal job structs; fixtures in `testdata/`. Phase 1.
