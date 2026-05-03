# Authentication and abuse prevention

How we protect the API (app-level) and limit cost (AWS and API Gateway). Assumes no prior AWS security experience.

---

## Two layers

| Layer | Question | Where it lives |
|-------|----------|----------------|
| **App authentication** | "Is this request from a caller we trust?" (e.g. Stripe) | Your code (e.g. verify Stripe signature) or API Gateway (API key, authorizer). |
| **AWS resource authorization** | "Who can invoke this Lambda or call this API?" | IAM and API Gateway (permissions, keys, throttling). |

Both matter. App auth decides whether you *process* a request as valid. AWS/auth and throttling decide who can *hit* the endpoint and how much they can do.

---

## How to stop "anyone" from making millions of invocations

### 1. Nobody can invoke your Lambda *directly* from the internet

Lambda has no public URL. The only ways to invoke it are:

- **API Gateway** (for the webhook/healthz URL you create), or
- **SQS** (for the worker), or
- **AWS CLI/Console** (your credentials).

So random people cannot call the Lambda by ARN or some "Lambda URL". They can only hit what you expose: the **API Gateway URL**. If they know or guess that URL, they can send HTTP requests to it, and **each request will cause API Gateway to invoke your Lambda**. So (1) alone does **not** stop abuse: it only stops *direct* invocation of the Lambda. To stop someone who gets their request to API Gateway from triggering your Lambda, you need step 2.

### 2. Restrict who can get past API Gateway (required to stop URL abuse)

If someone can get their request to API Gateway (they know the URL, or it leaks), API Gateway will invoke your Lambda unless you add a gate. Use one or both of:

- **API key** – API Gateway checks a header (e.g. `x-api-key`) and only forwards to Lambda if the key is valid. No key or wrong key: API Gateway returns 403 and **does not invoke** your Lambda. So unknown callers never trigger your function.
- **Lambda authorizer** – A small Lambda runs first and checks a header/token; it returns allow or deny. Only allowed requests are forwarded to your main Lambda. Denied requests do not invoke your ingest Lambda.

Without either, **anyone who has the API Gateway URL can invoke your Lambda** on every request. (Optional for Phase 0 learning; recommended as soon as you care about cost or abuse.)

Options (you can combine):

| Mechanism | What it does | When to use |
|-----------|---------------|-------------|
| **API key** | Only requests that send a valid key in a header (e.g. `x-api-key`) are passed to Lambda. Invalid or missing key: 403, Lambda not invoked. | Simple; good for a small set of callers. Key can leak, so rotate if needed. |
| **Lambda authorizer** | A small Lambda runs first. It checks a header/token (e.g. bearer token, custom secret). Returns allow/deny. Only allowed requests reach your ingest Lambda. | Flexible; you can check a shared secret or Stripe signature here. |
| **Stripe signature verification (in app)** | Your ingest Lambda verifies the `Stripe-Signature` header using your webhook secret. If invalid, return 401 and do not enqueue. | Required for production Stripe webhooks. Stops random POSTs from being treated as valid; they still *invoke* Lambda once. |

So: **signature verification** stops bad requests from being processed but they still count as one invocation. To reduce *invocations* from unknown callers, add an **API key** or **Lambda authorizer** so only requests that know the secret/key reach your Lambda.

---

## API key in practice: do we need it, and how?

**Do we need to implement it?** Not for Phase 0 (learning). Yes if you want to stop anyone with the URL from invoking your Lambda (e.g. before you add Stripe, or for healthz). You can add it in Phase 0 or when you add the webhook.

**Is it common?** Yes. Using a shared secret or API key to gate a webhook or internal API is standard. Stripe and many SaaS providers support sending a secret header; you can require that and optionally throttle by key.

**How it works with our stack (API Gateway HTTP API):**  
API Gateway **HTTP API** (what we use) does **not** have built-in “require API key” like the older REST API. So “API key” for us means:

- **Lambda authorizer** – A small Lambda that runs before your main Lambda. It receives the request, reads a header (e.g. `x-api-key` or `Authorization`), compares it to a secret you store (env var or AWS Systems Manager Parameter Store). If it matches, return “allow”; otherwise “deny”. API Gateway then either invokes your ingest Lambda or returns 403 without invoking it. So you do need a small amount of code: one authorizer Lambda (often a few lines of Go or Node) and Terraform to wire it.

**Does it require code?**  
- **With HTTP API:** Yes. You implement a Lambda authorizer (one function: read header, compare to secret, return allow/deny). No change to your main Lambda; the authorizer is a separate, small Lambda.  
- **With REST API instead of HTTP API:** API Gateway can require an API key natively (create key + usage plan in Terraform, require key on the API). No authorizer code. We chose HTTP API for simplicity and cost, so if we want key-based gating we use an authorizer.

**Summary:** For this project, “API key” = Lambda authorizer that checks a header (e.g. `x-api-key`) against a secret. One small authorizer Lambda + Terraform; no code in your healthz or ingest Lambda. Add it when you want to lock down the URL.

### 3. Throttle so one caller cannot do "millions"

- **API Gateway usage plans** – Throttle by API key (e.g. 1000 req/s, 10k/day). Even if someone gets your URL and key, they hit the cap.
- **Account-level throttles** – AWS has default limits (e.g. concurrent Lambda executions). Prevents unbounded concurrency but not total invocations.
- **Reserved concurrency (Lambda)** – Cap concurrent executions for a function. Protects downstream and limits blast radius; does not by itself block high request volume.

So: use **API key + usage plan** or **authorizer** to limit who can call, and **throttling** to cap how much.

### 4. Cost safety net

- **Billing alert** – In AWS Billing, set a budget (e.g. $5) and an email alert. You get notified before a surprise bill.
- **Lambda reserved concurrency** – Cap concurrency so one runaway caller does not spin up thousands of executions.

---

## Stripe webhooks: who gives what (API key vs signing secret)

When we subscribe to Stripe webhooks, Stripe needs our URL and we need a way to trust that requests are from Stripe.

- **We give Stripe:** Only our webhook URL (e.g. `https://xxx.execute-api.region.amazonaws.com/webhook`). We do not give Stripe an API key.
- **Stripe gives us:** A **webhook signing secret** (e.g. `whsec_...`) when we create the webhook endpoint in the Stripe Dashboard. We store it (e.g. env var) and use it in our ingest Lambda to verify the `Stripe-Signature` header on each request. If the signature is invalid, we return 401 and do not process the event.

So Stripe does not use an API key from us. We use the secret Stripe gave us to verify that the request came from Stripe. Stripe's webhook delivery does not support custom headers (like `x-api-key`), so we cannot have Stripe send "our" API key; signature verification is the standard and correct approach for Stripe webhooks.

---

## What we do in this project

| Phase | App auth | AWS / abuse |
|-------|----------|-------------|
| **Phase 0 (healthz)** | None (public GET for learning). Optional: require API key so only you can call. | Lambda permission: only API Gateway can invoke. Add billing alert. |
| **Ingest (webhook)** | Verify Stripe signature in the ingest handler; reject invalid. Optional: API key or authorizer so only Stripe (and your tests) can hit the URL. | Same: only API Gateway invokes Lambda. Throttling + usage plan when you add API key. |
| **Worker** | N/A (triggered by SQS, not public). | Only SQS (and your account) can invoke the worker Lambda. |

---

## IAM and "who can invoke my Lambda"

- **Lambda resource-based policy** – The `aws_lambda_permission` in Terraform says: "Allow API Gateway (this specific API) to invoke this Lambda." So only that API Gateway can invoke it; nobody can invoke the Lambda *directly* by ARN from the internet. But **anyone who can send HTTP requests to your API Gateway URL will still trigger the Lambda** unless you add an API key or authorizer on the API.
- **Lambda execution role** – The role the Lambda *runs as* (e.g. to write logs). It does not control who invokes the Lambda. So: execution role = what Lambda can do; resource policy = who is allowed to invoke it (API Gateway / SQS), not who can reach API Gateway.

**Bottom line:** IAM stops direct Lambda invocation. To stop "someone who gets their request to API Gateway" from invoking your Lambda, you must add API key or authorizer so API Gateway rejects them before calling your function.

---

## Summary

- **Stop millions of invocations:** (1) IAM ensures only API Gateway (and SQS) can invoke Lambda, so no *direct* invoke from the internet. (2) **If someone can reach your API Gateway URL, they will trigger your Lambda unless you add API key or authorizer** so API Gateway rejects them first. (3) Add API key or authorizer so only legitimate callers get through; use throttling/usage plans to cap rate. (4) Set a billing alert.
- **App authentication:** For Stripe, verify `Stripe-Signature` in the ingest handler. Optionally add API key or authorizer so only callers with the key/secret can reach the endpoint.
- **Phase 0:** Keeping healthz open is fine for learning. For production, add at least an API key and a billing alert; add signature verification when you add the webhook.
