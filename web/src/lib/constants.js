/** The pipeline, in order. The UI lights up however far a payment has reached. */
export const STAGES = [
  { name: 'API', detail: 'payment + outbox, one transaction' },
  { name: 'Outbox', detail: 'event queued, waiting for the relay' },
  { name: 'Kafka', detail: 'payment.created, keyed by payment id' },
  { name: 'Worker', detail: 'consumes it, charges the gateway' },
  { name: 'Gateway', detail: 'accepts — customer has not paid yet' },
  { name: 'Webhook', detail: 'settles it SUCCEEDED or FAILED' },
];

/** How many stages a status implies the payment has passed. */
export const STAGES_REACHED = {
  PENDING: 2,
  PROCESSING: 5,
  UNKNOWN: 4,
  SUCCEEDED: 6,
  FAILED: 6,
};

export const STATUS_ORDER = ['PENDING', 'PROCESSING', 'UNKNOWN', 'SUCCEEDED', 'FAILED'];

/** Maps a status onto the brand palette variable that carries its meaning. */
export const STATUS_COLOR_VAR = {
  PENDING: 'yellow',
  PROCESSING: 'turquoise',
  UNKNOWN: 'navy',
  SUCCEEDED: 'dark-green',
  FAILED: 'orange',
};

export const STATUS_HINT = {
  PENDING: 'The relay ships the outbox event within a second, then the worker charges the gateway.',
  PROCESSING:
    'The gateway has the charge. <b>Nothing moves until the customer pays</b> — press Pay or Fail on the row.',
  UNKNOWN:
    'The gateway never answered, so we do not know if the money moved. Reconciliation sweeps this and asks.',
  SUCCEEDED: 'Terminal. A later webhook for this payment is accepted with 200 but ignored.',
  FAILED: 'Terminal. Either the gateway declined it or the customer never paid.',
};

export const FAIL_MODES = [
  { value: '', label: 'Normal — accept the charge' },
  { value: 'decline', label: 'Decline — 402, payment FAILED' },
  { value: 'error', label: 'Error — 500, retried then UNKNOWN' },
  { value: 'timeout', label: 'Timeout — no answer, UNKNOWN' },
];

export const FAIL_MODE_TEXT = {
  '': 'Charges are accepted. The payment parks at <b>PROCESSING</b> until you settle it.',
  decline:
    'The gateway answers <b>402</b>. A refusal is an answer, so there is <b>no retry</b> — straight to FAILED.',
  error:
    'The gateway answers <b>500</b>. That failure is known not to have charged, so the worker <b>retries 3&times;</b>, then gives up to UNKNOWN.',
  timeout:
    'The gateway hangs. We never learn if money moved, so it is <b>never retried</b> — straight to UNKNOWN.',
};

export const PAYMENT_METHODS = [
  { value: 'va_bca', label: 'Virtual Account — BCA' },
  { value: 'va_bni', label: 'Virtual Account — BNI' },
  { value: 'qris', label: 'QRIS' },
  { value: 'card', label: 'Card' },
];

export const POLL_MS = 2000;
export const HEALTH_POLL_MS = 10000;
