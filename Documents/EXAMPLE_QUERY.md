# SSIS Packages — Recommended Features & Implementation Roadmap

This document lists recommended new features and hardening actions for the SSIS packages in `Documents/SSIS_EXAMPLES`. Items are grouped in phases by priority, with short implementation notes, estimated effort, and acceptance criteria.

**Copilot Query:** recommend and document new features in a markdown file under Documents. group in phases by priority.

**Scope:** Recommendations apply to the SSIS packages stored in `Documents/SSIS_EXAMPLES` (e.g. `Scanner.dtsx`, `RunMultu.dtsx`, `Loader.dtsx`, etc.). The recommendations assume packages will continue to run under the current orchestration environment.

---

**Phase 1 — High Priority (Immediate / 1–2 sprints)**

- **Logging & Centralized Monitoring:**

  - Description: Enable package-level logging (OnError, OnWarning, OnPreExecute, OnPostExecute) and forward logs to a central store (SSISDB catalog logging, database table, or ELK/Datadog).
  - Steps: enable SSIS logging in each package or configure project-level logging; create a reusable logging table or use SSISDB built-in logging.
  - Estimated effort: 4–8h per package (smaller packages less).
  - Acceptance criteria: package logs show start/stop and any OnError entries in the central store for test runs.

- **Event Handlers & Error Quarantine:**

  - Description: Add `OnError` handlers that record task name, error code, message, package name, execution id and optionally route failed rows to a quarantine table/file.
  - Steps: add an `OnError` handler at package level; implement an Execute SQL Task to insert failure details into a `package_errors` table; for Data Flow tasks enable error outputs and write quarantined rows to a file or table.
  - Estimated effort: 3–6h per package with Data Flow.
  - Acceptance criteria: simulated failures generate error rows and create entries in the `package_errors` table.

- **Harden Connection Managers & Secrets:**

  - Description: Remove embedded credentials from connection strings; use project parameters/environment variables or SSISDB sensitive parameters; consider Key Vault for secrets.
  - Steps: audit connection managers for `User ID`/password fragments; convert sensitive values to parameters and mark as sensitive; configure SSIS Catalog environment references.
  - Estimated effort: 1–3h per package.
  - Acceptance criteria: no connection manager contains plain-text credentials; deployment uses parameters or protect-level settings.

- **Eliminate or Secure Clear-text Cache Files:**

  - Description: Where Cache Transform (`.caw`) is used, ensure it does not contain PII or secrets. If required, move to secure store or avoid caching sensitive columns.
  - Steps: identify packages using Cache Transform (e.g., `Scanner.dtsx`), review cached columns, remove sensitive columns or encrypt cache files externally.
  - Estimated effort: 2–4h per affected package.
  - Acceptance criteria: no `.caw` file contains sensitive columns in plaintext.

- **Lookup SQL Improvements:**
  - Description: Replace `SELECT *` used in Lookups with explicit column lists and add filters to minimize memory usage and schema coupling.
  - Steps: update Lookup queries to explicitly select required columns and add WHERE clause if appropriate.
  - Estimated effort: 1–2h per Lookup.
  - Acceptance criteria: Lookups use explicit column lists and pass integration tests.

---

**Phase 2 — Medium Priority (Next 2–4 sprints)**

- **Parameterize & Support Multiple Environments:**

  - Description: Introduce project and package parameters for environment-specific values (connection names, file paths, dates). Deploy via SSIS Catalog Environments.
  - Steps: add project parameters; map package parameters; create SSISDB environments for dev/test/prod.
  - Estimated effort: 1–2 days to roll out across the project.
  - Acceptance criteria: the same package deploys to dev/test/prod using different environment values without code changes.

- **Secrets Management (Key Vault / Managed Service):**

  - Description: Integrate Azure Key Vault or equivalent for storing DB credentials and sensitive tokens, retrieving them at runtime.
  - Steps: implement a small .NET assembly or Script Task that fetches secrets securely, or leverage SSISDB sensitive parameters with restricted access.
  - Estimated effort: 2–4 days (including security review).
  - Acceptance criteria: no environment requires embedding secrets in `.dtsx` files; secrets pulled securely at runtime.

- **Automated Validation & Unit Tests:**

  - Description: Add deployment-time validation tests (small test packages or query-level checks) and CI build checks.
  - Steps: create smoke tests for critical data flows; integrate into CI pipeline to run tests after deployment.
  - Estimated effort: 3–6 days to create baseline tests and pipeline steps.
  - Acceptance criteria: CI runs smoke tests and fails on regressions.

- **Data Flow Hardening (Error Outputs, Row Counts):**
  - Description: Standardize error outputs, add row-count checks and thresholds to detect anomalies.
  - Steps: add Row Count transforms and conditional checks; record row counts to monitoring store.
  - Estimated effort: 1–3 days across packages with Data Flow.
  - Acceptance criteria: anomaly alerts generated when row counts deviate significantly.

---

**Phase 3 — Low Priority (Backlog / Future)**

- **Performance Tuning & Observability:**

  - Description: Add detailed timings, task-level metrics, and consider parallelization where safe.
  - Steps: instrument packages to measure duration per task; profile long-running tasks and optimize queries/indices.
  - Estimated effort: per-package, varies.

- **CI/CD and Packaging:**

  - Description: Add packaging, automated deployments to SSIS Catalog, and versioning strategies.
  - Steps: create deployment scripts, adopt semantic versioning for packages.

- **Documentation & Runbooks:**
  - Description: Create operational runbooks for each package describing purpose, inputs, outputs, dependencies, and troubleshooting steps.
  - Steps: generate a template and fill it per package; store alongside packages in `Documents` or repo.

---

**Quick Implementation Examples**

- **Sample OnError handler (conceptual steps):**

  1. Add an `OnError` event handler at the package scope.
  2. Add an `Execute SQL Task` in the handler pointing at the monitoring DB.
  3. SQL Task inserts into `package_errors` with columns: `execution_id`, `package_name`, `task_name`, `error_code`, `message`, `occurred_at`.

- **Fixing `SELECT *` in Lookup (example):**

  - Bad: `SELECT * FROM [Production].[Product]`
  - Better: `SELECT ProductID, ProductName, ProductCategory, Price FROM [Production].[Product] WHERE IsActive = 1`

- **Enable Catalog-Level Logging (SSISDB):**
  - Steps: deploy project to SSISDB; configure SSISDB logging levels and create an SSISDB environment with values for parameters; enable retention policies.

---

**Acceptance & Rollout Recommendations**

- Start with Phase 1 on the smallest number of high-impact packages (e.g., packages that run nightly or touch sensitive data). Validate changes in a dev environment.
- Use feature branches and small PRs that change one aspect (logging, lookup, or cache) per PR for easier review.
- Keep a short runbook for each package updated as changes are rolled out.

---

**Next Steps**

- Confirm priority packages to start (I can help pick the top 2–3).
- If you'd like, I can open PRs with suggested changes for a single package (example: add `OnError` handler and convert `select *` lookup in `Scanner.dtsx`).

**Document owner:** create/maintain this file in `Documents/SSIS_FEATURES_RECOMMENDATIONS.md`.
