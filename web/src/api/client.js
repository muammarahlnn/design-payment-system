const PAYMENTS = '/api/payments';
const GATEWAY = '/api/gateway';

/**
 * Thin fetch wrapper. Throws an Error carrying `status` so callers can branch
 * on the HTTP code (a 404 from the gateway means something specific) without
 * every component re-parsing responses.
 */
async function request(url, options = {}) {
  const res = await fetch(url, options);
  const text = await res.text();

  let body = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = { raw: text };
  }

  if (!res.ok) {
    const error = new Error(body?.error || body?.raw || `HTTP ${res.status}`);
    error.status = res.status;
    throw error;
  }
  return { body, status: res.status };
}

const json = { 'Content-Type': 'application/json' };

export const paymentsApi = {
  health: () => request(`${PAYMENTS}/health`),

  list: (limit = 60, signal) =>
    request(`${PAYMENTS}/payments?limit=${limit}`, { signal }).then((r) => r.body.payments ?? []),

  summary: (signal) =>
    request(`${PAYMENTS}/payments/summary`, { signal }).then((r) => r.body.counts ?? {}),

  detail: (id, signal) => request(`${PAYMENTS}/payments/${id}`, { signal }).then((r) => r.body),

  /** Returns the body plus the status, because 201 vs 200 is the whole lesson. */
  create: ({ customerId, amount, paymentMethod, idempotencyKey }) =>
    request(`${PAYMENTS}/payments`, {
      method: 'POST',
      headers: { ...json, 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify({
        customer_id: customerId,
        amount: Number(amount),
        payment_method: paymentMethod,
      }),
    }),
};

export const gatewayApi = {
  health: () => request(`${GATEWAY}/health`),

  getConfig: (signal) => request(`${GATEWAY}/config`, { signal }).then((r) => r.body),

  setConfig: (failMode) =>
    request(`${GATEWAY}/config`, {
      method: 'PUT',
      headers: json,
      body: JSON.stringify({ fail_mode: failMode }),
    }).then((r) => r.body),

  /** Stands in for the customer completing or abandoning the payment. */
  settle: (paymentId, status) =>
    request(`${GATEWAY}/charges/${paymentId}/settle`, {
      method: 'POST',
      headers: json,
      body: JSON.stringify({ status }),
    }).then((r) => r.body),
};
