import { TopBar } from '../components/TopBar';

/**
 * The system-design write-up. Static content, so it is plain JSX with no state
 * and no effects — the diagrams are inline SVG rather than images so they stay
 * crisp and pick up the brand palette from CSS variables.
 */
export function Learn() {
  return (
    <>
      <TopBar subtitle="system design, worked end to end" />
      <div className="learn">


  <p className="lead" style={{marginBottom: '22px'}}>
    A payment system is the standard interview question for <b>exactly-once semantics in a
    distributed system</b>, because money makes every failure mode expensive. This page is the
    design behind the playground: what it must guarantee, how each guarantee is bought, and
    what it costs.
  </p>

  <div className="toc">
    <b>On this page</b>
    <ol>
      <li><a href="#problem">The problem</a></li>
      <li><a href="#reqs">Requirements</a></li>
      <li><a href="#arch">Architecture</a></li>
      <li><a href="#dual">The core problem: dual writes</a></li>
      <li><a href="#states">The state machine</a></li>
      <li><a href="#idem">Four layers of idempotency</a></li>
      <li><a href="#failures">Failure modes</a></li>
      <li><a href="#tradeoffs">Trade-offs</a></li>
      <li><a href="#scale">Scaling</a></li>
      <li><a href="#interview">Interview answers</a></li>
    </ol>
  </div>

  {/* 1 */}
  <section id="problem">
    <h2><span className="num">01</span> The problem</h2>
    <p>
      Taking a payment means coordinating three systems that can each fail independently:
      <b>your database</b>, <b>a message broker</b>, and <b>a payment provider you do not
      control</b>. None of them can participate in a shared transaction.
    </p>
    <p>Every hard question in this design comes from one fact:</p>
    <div className="callout">
      <span className="lbl">The fact everything follows from</span>
      When a network call times out, <b>you cannot tell whether it succeeded.</b> The request may
      have arrived and been processed, or never arrived at all. Both look identical to the caller.
    </div>
    <p>
      If you could tell, you would simply retry when it failed. You cannot, so the system has to
      be built so that retrying is <em>always</em> safe.
    </p>
  </section>

  {/* 2 */}
  <section id="reqs">
    <h2><span className="num">02</span> Requirements</h2>
    <div className="two">
      <div className="panel">
        <h3 style={{marginTop: '0'}}>Functional</h3>
        <ul>
          <li>Accept a payment request</li>
          <li>Submit the charge to a provider</li>
          <li>Track it to a terminal state</li>
          <li>Accept provider callbacks</li>
          <li>Let operators see what happened</li>
        </ul>
      </div>
      <div className="panel">
        <h3 style={{marginTop: '0'}}>Non-functional &mdash; the real question</h3>
        <ul>
          <li><b>Never charge twice</b>, under any retry</li>
          <li><b>Never lose a payment</b>, under any crash</li>
          <li>Survive provider downtime</li>
          <li>Survive our own process dying mid-flight</li>
          <li>Every state change auditable</li>
        </ul>
      </div>
    </div>
    <div className="callout navy">
      <span className="lbl">Scope taken here</span>
      IDR only, treated as zero-decimal. One merchant. Single Kafka partition. No refunds,
      partial captures or 3DS. Those are deliberate cuts, not oversights &mdash; each is called
      out where it would change the design.
    </div>
  </section>

  {/* 3 */}
  <section id="arch">
    <h2><span className="num">03</span> Architecture</h2>
    <div className="panel fig">
      <svg viewBox="0 0 920 540" width="920" height="540" role="img" aria-label="Architecture diagram">
        <defs>
          <marker id="ar" markerWidth="11" markerHeight="11" refX="9" refY="4" orient="auto">
            <path d="M0,0 L9,4 L0,8" fill="none" stroke="#5b5878" strokeWidth="1.6" strokeLinecap="round"/>
          </marker>
          <marker id="aro" markerWidth="11" markerHeight="11" refX="9" refY="4" orient="auto">
            <path d="M0,0 L9,4 L0,8" fill="none" stroke="#f15a22" strokeWidth="1.8" strokeLinecap="round"/>
          </marker>
        </defs>
        <style>{`
          .bx { fill:#fff; stroke:#262262; stroke-width:2; rx:12 }
          .bx.g { fill:#f1f9f8; stroke:#10565f }
          .bx.o { fill:#fff5ef; stroke:#f15a22 }
          .bx.y { fill:#fffaf2; stroke:#f7941d }
          .tl { font:700 13px Mulish,sans-serif; fill:#262262 }
          .sl { font:400 11px Mulish,sans-serif; fill:#5b5878 }
          .hand { font:500 15px Caveat,cursive; fill:#f15a22 }
          .ln { stroke:#5b5878; stroke-width:1.6; fill:none; marker-end:url(#ar) }
          .ln.o { stroke:#f15a22; stroke-width:1.8; marker-end:url(#aro) }
          .ln.d { stroke-dasharray:5 4 }
        `}</style>

        {/* client */}
        <rect className="bx" x="20" y="24" width="120" height="46"/>
        <text className="tl" x="80" y="45" textAnchor="middle">Client</text>
        <text className="sl" x="80" y="60" textAnchor="middle">sends Idempotency-Key</text>

        {/* payment-service */}
        <rect className="bx o" x="200" y="18" width="180" height="58"/>
        <text className="tl" x="290" y="41" textAnchor="middle">payment-service</text>
        <text className="sl" x="290" y="57" textAnchor="middle">POST /payments</text>
        <text className="sl" x="290" y="69" textAnchor="middle">POST /webhook/payment</text>
        <path className="ln" d="M142 47 H196"/>

        {/* postgres: one transaction */}
        <rect className="bx" x="180" y="112" width="220" height="112" fill="#faf9fe"/>
        <text className="tl" x="290" y="133" textAnchor="middle">Postgres</text>
        <rect x="196" y="143" width="188" height="22" rx="6" fill="#fff" stroke="#e7e5f0"/>
        <text className="sl" x="290" y="158" textAnchor="middle">payments</text>
        <rect x="196" y="168" width="188" height="22" rx="6" fill="#fff" stroke="#e7e5f0"/>
        <text className="sl" x="290" y="183" textAnchor="middle">idempotency_keys</text>
        <rect x="196" y="193" width="188" height="22" rx="6" fill="#fff" stroke="#e7e5f0"/>
        <text className="sl" x="290" y="208" textAnchor="middle">outbox</text>
        <path className="ln o" d="M290 80 V108"/>
        <text className="hand" x="300" y="100">one transaction</text>

        {/* relay */}
        <rect className="bx y" x="196" y="258" width="188" height="42"/>
        <text className="tl" x="290" y="275" textAnchor="middle">outbox relay</text>
        <text className="sl" x="290" y="291" textAnchor="middle">FOR UPDATE SKIP LOCKED, 1s</text>
        <path className="ln" d="M290 228 V254"/>

        {/* kafka */}
        <rect className="bx" x="196" y="334" width="188" height="46"/>
        <text className="tl" x="290" y="353" textAnchor="middle">Kafka</text>
        <text className="sl" x="290" y="369" textAnchor="middle">payment.created &middot; key = payment id</text>
        <path className="ln" d="M290 304 V330"/>

        {/* worker */}
        <rect className="bx g" x="196" y="414" width="188" height="52"/>
        <text className="tl" x="290" y="434" textAnchor="middle">payment-worker</text>
        <text className="sl" x="290" y="450" textAnchor="middle">consumes + charges</text>
        <text className="sl" x="290" y="462" textAnchor="middle">inbox dedup</text>
        <path className="ln" d="M290 384 V410"/>

        {/* gateway */}
        <rect className="bx" x="560" y="414" width="180" height="52"/>
        <text className="tl" x="650" y="434" textAnchor="middle">Payment gateway</text>
        <text className="sl" x="650" y="450" textAnchor="middle">idempotent by key</text>
        <text className="sl" x="650" y="462" textAnchor="middle">VA / QRIS / card</text>
        <path className="ln o" d="M388 440 H556"/>
        <text className="hand" x="400" y="432">charge, key = payment id</text>

        {/* webhook back */}
        <path className="ln o d" d="M650 410 V120 Q650 96 626 96 H386"/>
        <text className="hand" x="470" y="88">webhook: customer paid</text>

        {/* reconciler */}
        <rect className="bx y" x="560" y="258" width="180" height="60"/>
        <text className="tl" x="650" y="278" textAnchor="middle">reconciler</text>
        <text className="sl" x="650" y="294" textAnchor="middle">sweeps stuck payments</text>
        <text className="sl" x="650" y="306" textAnchor="middle">asks the gateway directly</text>
        <path className="ln d" d="M650 320 V406"/>
        <path className="ln d" d="M556 288 H404 Q392 288 392 262 V236"/>
        <text className="hand" x="430" y="256">repairs what was lost</text>

        {/* legend */}
        <rect x="788" y="24" width="112" height="86" rx="10" fill="#faf9fe" stroke="#e7e5f0"/>
        <text className="sl" x="800" y="42" style={{fontWeight: '700'}}>legend</text>
        <path className="ln" d="M800 56 H830"/><text className="sl" x="836" y="60">sync</text>
        <path className="ln d" d="M800 76 H830"/><text className="sl" x="836" y="80">async</text>
        <path className="ln o" d="M800 96 H830"/><text className="sl" x="836" y="100">money</text>
      </svg>
      <div className="cap">Three independent systems. No shared transaction anywhere.</div>
    </div>

    <table>
      <thead><tr><th>Component</th><th>Job</th><th>If it dies</th></tr></thead>
      <tbody>
        <tr><td><code>payment-service</code></td><td>accept requests, receive callbacks</td><td>clients retry with the same key; nothing is lost</td></tr>
        <tr><td><code>outbox relay</code></td><td>ship queued events to Kafka</td><td>events sit in the table until it returns</td></tr>
        <tr><td><code>payment-worker</code></td><td>consume events, charge the provider</td><td>Kafka holds the offset; it resumes</td></tr>
        <tr><td><code>reconciler</code></td><td>repair whatever the above lost</td><td>stuck payments stay stuck, nothing corrupts</td></tr>
      </tbody>
    </table>
  </section>

  {/* 4 */}
  <section id="dual">
    <h2><span className="num">04</span> The core problem: dual writes</h2>
    <p>
      Creating a payment means doing two things: writing a row, and telling the rest of the system
      about it. They live in different systems, so they cannot be one transaction. There are only
      three options, and two of them are wrong.
    </p>
    <div className="panel fig">
      <svg viewBox="0 0 900 230" width="900" height="230" role="img" aria-label="Dual write options">
        <style>{`
          .h  { font:700 13px Mulish,sans-serif; fill:#262262 }
          .s  { font:400 11.5px Mulish,sans-serif; fill:#5b5878 }
          .bad{ font:700 12px Mulish,sans-serif; fill:#f15a22 }
          .ok { font:700 12px Mulish,sans-serif; fill:#10565f }
          .stp{ fill:#fff; stroke:#c9c6dd; stroke-width:1.6; rx:8 }
          .boom { stroke:#f15a22; stroke-width:2; fill:none; stroke-linecap:round }
        `}</style>

        {/* option 1 */}
        <text className="h" x="16" y="22">1. Publish, then commit</text>
        <rect className="stp" x="16"  y="34" width="104" height="30"/><text className="s" x="68"  y="54" textAnchor="middle">publish event</text>
        <path className="boom" d="M128 49 h22"/><path className="boom" d="M139 41 v16"/>
        <rect className="stp" x="158" y="34" width="104" height="30"/><text className="s" x="210" y="54" textAnchor="middle">commit row</text>
        <text className="bad" x="276" y="53">crash here &rarr; an event for a payment that does not exist</text>

        {/* option 2 */}
        <text className="h" x="16" y="104">2. Commit, then publish</text>
        <rect className="stp" x="16"  y="116" width="104" height="30"/><text className="s" x="68"  y="136" textAnchor="middle">commit row</text>
        <path className="boom" d="M128 131 h22"/><path className="boom" d="M139 123 v16"/>
        <rect className="stp" x="158" y="116" width="104" height="30"/><text className="s" x="210" y="136" textAnchor="middle">publish event</text>
        <text className="bad" x="276" y="135">crash here &rarr; a payment nobody ever hears about</text>

        {/* option 3 */}
        <text className="h" x="16" y="186">3. Outbox</text>
        <rect className="stp" x="16" y="198" width="246" height="30" style={{fill: '#f1f9f8', stroke: '#10565f'}}/>
        <text className="s" x="139" y="218" textAnchor="middle">commit row + event together</text>
        <path d="M270 213 h26" stroke="#10565f" strokeWidth="1.6" fill="none"/>
        <rect className="stp" x="304" y="198" width="104" height="30" style={{fill: '#f1f9f8', stroke: '#10565f'}}/>
        <text className="s" x="356" y="218" textAnchor="middle">relay ships it</text>
        <text className="ok" x="422" y="217">no crash window &mdash; one system, one transaction</text>
      </svg>
      <div className="cap">You cannot make two systems atomic. You can avoid needing to.</div>
    </div>
    <div className="callout green">
      <span className="lbl">Say this in an interview</span>
      <b>The outbox does not make delivery reliable &mdash; it makes the database the single source of
      truth and lets delivery be unreliable.</b> The relay can publish the same event five times;
      consumers deduplicate. What it can never do is lose one.
    </div>
    <p>
      This was not theoretical here. A disk-full incident mid-project destroyed every Kafka message
      and every consumer offset. <b>Not one payment was lost</b>, because Postgres held the truth and
      the relay replayed from the outbox.
    </p>
  </section>

  {/* 5 */}
  <section id="states">
    <h2><span className="num">05</span> The state machine</h2>
    <div className="panel fig">
      <svg viewBox="0 0 880 300" width="880" height="300" role="img" aria-label="Payment state machine">
        <defs>
          <marker id="a2" markerWidth="11" markerHeight="11" refX="9" refY="4" orient="auto">
            <path d="M0,0 L9,4 L0,8" fill="none" stroke="#5b5878" strokeWidth="1.6" strokeLinecap="round"/>
          </marker>
        </defs>
        <style>{`
          .st { rx:22; stroke-width:2.5 }
          .sn { font:800 13px Mulish,sans-serif; fill:#fff }
          .ed { stroke:#5b5878; stroke-width:1.6; fill:none; marker-end:url(#a2) }
          .el { font:500 14px Caveat,cursive; fill:#5b5878 }
          .note { font:400 11px Mulish,sans-serif; fill:#5b5878 }
        `}</style>

        <rect className="st" x="24"  y="120" width="128" height="44" fill="#f7941d"/>
        <text className="sn" x="88"  y="147" textAnchor="middle">PENDING</text>

        <rect className="st" x="256" y="40"  width="140" height="44" fill="#32bcad"/>
        <text className="sn" x="326" y="67"  textAnchor="middle">PROCESSING</text>

        <rect className="st" x="256" y="206" width="140" height="44" fill="#262262"/>
        <text className="sn" x="326" y="233" textAnchor="middle">UNKNOWN</text>

        <rect className="st" x="560" y="40"  width="140" height="44" fill="#10565f"/>
        <text className="sn" x="630" y="67"  textAnchor="middle">SUCCEEDED</text>

        <rect className="st" x="560" y="206" width="140" height="44" fill="#f15a22"/>
        <text className="sn" x="630" y="233" textAnchor="middle">FAILED</text>

        <path className="ed" d="M156 132 Q206 120 252 82"/>
        <text className="el" x="162" y="108">accepted</text>

        <path className="ed" d="M156 156 Q206 182 252 216"/>
        <text className="el" x="150" y="196">no answer</text>

        <path className="ed" d="M120 166 Q190 282 554 240"/>
        <text className="el" x="250" y="276">declined &mdash; no retry</text>

        <path className="ed" d="M400 62 H554"/>
        <text className="el" x="432" y="52">webhook</text>

        <path className="ed" d="M398 76 Q480 150 556 212"/>
        <text className="el" x="430" y="150">webhook</text>

        <path className="ed" d="M398 218 Q470 190 556 78"/>
        <text className="el" x="412" y="190">reconciled</text>

        <path className="ed" d="M400 232 H554"/>
        <text className="el" x="430" y="256">reconciled</text>

        <path className="ed" d="M326 202 V90" strokeDasharray="5 4"/>
        <text className="el" x="334" y="150">gateway confirms</text>

        <text className="note" x="560" y="110">terminal &mdash; nothing moves out</text>
        <text className="note" x="560" y="276">terminal &mdash; nothing moves out</text>
      </svg>
      <div className="cap">Two terminal states. Everything else is a waiting room.</div>
    </div>

    <div className="callout">
      <span className="lbl">Why UNKNOWN is its own state</span>
      After a timeout, leaving a payment at <code>PENDING</code> is <b>a lie</b>. <code>PENDING</code>
      means "not submitted", and anything reading it &mdash; a human, a retry job, a dashboard &mdash;
      will conclude a charge is safe. It is not. The money may already have moved.
      <code>UNKNOWN</code> makes the dangerous case loud instead of hiding it inside a status that
      looks fine.
    </div>

    <p>
      Terminal states are also the <b>ordering rule</b>. Every settlement goes through one guard:
    </p>
    <pre><code>UPDATE payments SET status = $2
WHERE id = $1 AND status NOT IN ('SUCCEEDED','FAILED')</code></pre>
    <p>
      The webhook and the reconciler can both try to settle the same payment at the same instant.
      Whichever commits first wins; the loser's <code>UPDATE</code> matches zero rows and it stands
      down. No locks, no coordination, no leader election &mdash; and no version numbers needed,
      because with one hop and two terminal states, "never leave a terminal state" is the whole rule.
    </p>
  </section>

  {/* 6 */}
  <section id="idem">
    <h2><span className="num">06</span> Four layers of idempotency</h2>
    <p>
      People answer "use an idempotency key" and stop. That covers one of four places a retry can
      duplicate work. Each layer protects a different boundary.
    </p>
    <table>
      <thead><tr><th>#</th><th>Boundary</th><th>Mechanism</th><th>Stops</th></tr></thead>
      <tbody>
        <tr><td>1</td><td>Client &rarr; API</td><td><code>idempotency_keys.key</code> PRIMARY KEY + <code>ON CONFLICT DO NOTHING</code></td><td>a network retry creating two payments</td></tr>
        <tr><td>2</td><td>Worker &rarr; gateway</td><td><code>Idempotency-Key: &lt;payment id&gt;</code></td><td>a retried charge taking money twice</td></tr>
        <tr><td>3</td><td>Kafka &rarr; worker</td><td><code>processed_events (consumer, event_id)</code></td><td>a redelivered event being applied twice</td></tr>
        <tr><td>4</td><td>Any status write</td><td>compare-and-set in the <code>WHERE</code> clause</td><td>a stale writer reopening a settled payment</td></tr>
      </tbody>
    </table>

    <div className="callout navy">
      <span className="lbl">The line that matters</span>
      <b>Postgres enforces idempotency, not application code.</b> Check-then-insert races: two
      concurrent requests both find nothing and both insert. Putting the key in a unique index
      makes the database pick the winner. Tested with 30 parallel requests on one key &mdash;
      1&times; 201, 29&times; 200, one row.
    </div>

    <h3>Exactly-once does not exist. Exactly-once <em>effect</em> does.</h3>
    <p>
      Every hop here is <b>at-least-once</b>: the relay can publish twice, Kafka can redeliver,
      the worker can retry. What the four layers buy is that doing any of it twice has the same
      result as doing it once. That distinction is the single best thing to say when someone asks
      about exactly-once delivery &mdash; claiming you achieved it end to end is a red flag.
    </p>
  </section>

  {/* 7 */}
  <section id="failures">
    <h2><span className="num">07</span> Failure modes</h2>
    <p>Walk this table in an interview and you have covered the whole design.</p>
    <table>
      <thead><tr><th>What fails</th><th>What happens</th><th>What catches it</th></tr></thead>
      <tbody>
        <tr><td>Client retries after a timeout</td><td>same key arrives twice</td><td>unique index &rarr; 200 + the same payment</td></tr>
        <tr><td>Crash before commit</td><td>nothing happened</td><td>client retries; no trace left</td></tr>
        <tr><td>Crash after commit, before relay</td><td>event sits in the outbox</td><td>relay picks it up next tick</td></tr>
        <tr><td>Relay published, then failed to commit</td><td>event sent twice</td><td>inbox dedup on <code>event_id</code></td></tr>
        <tr><td>Kafka loses everything</td><td>all events and offsets gone</td><td>outbox replays &mdash; <b>this happened</b></td></tr>
        <tr><td>Gateway returns 5xx</td><td>known not to have charged</td><td>retry 3&times; with backoff</td></tr>
        <tr><td>Gateway times out</td><td>unknown whether it charged</td><td><code>UNKNOWN</code>, <b>no retry</b>, reconciler asks later</td></tr>
        <tr><td>Crash after charging, before recording</td><td>money moved, we do not know</td><td>event redelivered &rarr; gateway replays the same charge</td></tr>
        <tr><td>Webhook never arrives</td><td>stuck at <code>PROCESSING</code></td><td>reconciler asks the gateway directly</td></tr>
        <tr><td>Webhook arrives twice</td><td>duplicate settlement</td><td>inbox + terminal-state guard</td></tr>
        <tr><td>Webhook is forged</td><td>attacker settles a payment</td><td>HMAC signature + timestamp window</td></tr>
        <tr><td>Gateway says "no record"</td><td>ambiguous</td><td><b>nothing automatic</b> &mdash; flagged for a human</td></tr>
      </tbody>
    </table>

    <div className="callout">
      <span className="lbl">The two rows people miss</span>
      <b>Timeout is never retried inline.</b> Retrying is safe from a double-charge standpoint &mdash;
      the key sees to that &mdash; but the first request may still be executing, a provider that is
      timing out is already overloaded, and a retry will almost certainly time out too. You burn
      another full timeout to learn nothing.<br /><br />
      <b>"No record" is never acted on.</b> "That charge never existed" and "I lost my records" look
      identical over HTTP. Acting on the second one either loses a real payment or charges someone
      twice. A human decides.
    </div>

    <h3>Order of operations in the worker</h3>
    <p>Call the gateway <b>before</b> writing anything. The asymmetry decides it:</p>
    <table>
      <thead><tr><th>Order</th><th>A crash leaves</th><th>Recoverable?</th></tr></thead>
      <tbody>
        <tr><td><b>call, then record</b></td><td>charged but not recorded</td><td><b>yes</b> &mdash; redelivery replays the same charge, then records it</td></tr>
        <tr><td>record, then call</td><td>recorded but not charged</td><td><b>no</b> &mdash; the inbox marker already says "done"</td></tr>
      </tbody>
    </table>
    <p>
      It also keeps a database transaction from staying open across an HTTP call, which is how you
      exhaust a connection pool the first time the provider gets slow.
    </p>
  </section>

  {/* 8 */}
  <section id="tradeoffs">
    <h2><span className="num">08</span> Trade-offs</h2>
    <p>Interviewers care more about these than about the happy path.</p>
    <table>
      <thead><tr><th>Decision</th><th>Chosen</th><th>Cost</th><th>Alternative</th></tr></thead>
      <tbody>
        <tr><td>Dual write</td><td>outbox table</td><td>extra table, poll latency, rows to prune</td><td>CDC (Debezium) &mdash; lower latency, much more to operate</td></tr>
        <tr><td>Relay trigger</td><td>poll every 1s</td><td>up to 1s delay, constant query load</td><td>LISTEN/NOTIFY or CDC</td></tr>
        <tr><td>Settlement</td><td>async, via webhook</td><td>a long-lived <code>PROCESSING</code> state</td><td>synchronous &mdash; only works for cards, not VA or QRIS</td></tr>
        <tr><td>Ambiguity</td><td>explicit <code>UNKNOWN</code></td><td>one more state to handle</td><td><code>PROCESSING</code> + NULL reference &mdash; hides it</td></tr>
        <tr><td>Ordering</td><td>partition key = payment id</td><td>cannot add partitions later</td><td>version numbers per aggregate</td></tr>
        <tr><td>Money type</td><td><code>BIGINT</code>, whole rupiah</td><td>per-currency exponent logic later</td><td><code>NUMERIC</code> &mdash; exact but not what gateways accept</td></tr>
        <tr><td>Concurrency control</td><td>compare-and-set in <code>WHERE</code></td><td>loser must handle "0 rows"</td><td><code>SELECT FOR UPDATE</code> &mdash; a lock and a round trip</td></tr>
      </tbody>
    </table>
    <div className="callout navy">
      <span className="lbl">On money and floats</span>
      <code>BIGINT</code> in whole rupiah, never a float: <code>0.10</code> has no exact binary
      representation. And <code>INT</code> is a <b>bug</b> here, not a limit &mdash; it overflows at
      about Rp&nbsp;2,1&nbsp;miliar, which is a normal property or B2B transfer. In USD the same
      mistake hides until production, because <code>INT</code> holds $21M in cents.
    </div>
  </section>

  {/* 9 */}
  <section id="scale">
    <h2><span className="num">09</span> Scaling</h2>
    <table>
      <thead><tr><th>Pressure</th><th>What breaks first</th><th>Fix</th></tr></thead>
      <tbody>
        <tr><td>More API traffic</td><td>nothing &mdash; stateless</td><td>add replicas</td></tr>
        <tr><td>More replicas</td><td>N relays reorder events across batches</td><td>leader election, or a dedicated relay deployment</td></tr>
        <tr><td>More events</td><td>one partition caps consumer parallelism</td><td>more partitions &mdash; but see below</td></tr>
        <tr><td>Table growth</td><td><code>outbox</code>, <code>processed_events</code>, <code>idempotency_keys</code> grow forever</td><td>retention job or partition-and-drop</td></tr>
        <tr><td>Stuck payments</td><td>reconciler scan</td><td>already indexed; it is O(stuck), not O(all)</td></tr>
      </tbody>
    </table>
    <div className="callout">
      <span className="lbl">The partition trap</span>
      <code>hash(key) % 3</code> and <code>hash(key) % 6</code> give different answers.
      <b>Adding partitions to a topic that depends on key ordering breaks it permanently</b> &mdash;
      existing keys remap, and no replay fixes it. Over-provision at creation, or move ordering
      into the data with version numbers so the topic layout stops mattering.
    </div>
  </section>

  {/* 10 */}
  <section id="interview">
    <h2><span className="num">10</span> Interview answers</h2>

    <div className="qa">
      <div className="q">How do you stop double charges?</div>
      <div className="a">Four layers, not one. The client's key is a unique index so Postgres picks the
      winner under concurrency. The worker sends the payment id as the gateway's idempotency key, so
      a retried charge replays the first one. Consumed events are deduplicated in an inbox table
      written in the same transaction as their effects. And every status write is a compare-and-set,
      so a stale writer cannot reopen a settled payment.</div>
    </div>

    <div className="qa">
      <div className="q">Can you guarantee exactly-once?</div>
      <div className="a">Not delivery &mdash; nobody can across systems. Exactly-once <em>effect</em>, yes.
      Every hop is at-least-once and every operation is idempotent, so doing it twice equals doing
      it once.</div>
    </div>

    <div className="qa">
      <div className="q">The gateway times out. What now?</div>
      <div className="a">The charge may or may not have happened, so the payment goes to
      <code>UNKNOWN</code> &mdash; never back to <code>PENDING</code>, which would claim nothing
      happened. It is not retried inline: the first request may still be executing, and a provider
      that is timing out is already overloaded. Reconciliation asks the gateway later, once it has
      recovered.</div>
    </div>

    <div className="qa">
      <div className="q">The webhook never arrives.</div>
      <div className="a">That is why reconciliation exists. Every other path is event-driven, and
      event-driven systems lose events. The reconciler depends on nothing being delivered: it polls
      our own database for payments stuck past a threshold and asks the provider directly, because
      the provider is the authority on whether money moved.</div>
    </div>

    <div className="qa">
      <div className="q">Two webhooks arrive out of order.</div>
      <div className="a">Terminal states are the ordering rule. Settlement is guarded by
      <code>WHERE status NOT IN ('SUCCEEDED','FAILED')</code>, so the first terminal answer wins and
      anything later matches zero rows. For a longer lifecycle I would put a version number on each
      event and track the last applied version, which survives repartitioning and multiple topics.</div>
    </div>

    <div className="qa">
      <div className="q">How do you keep events ordered in Kafka?</div>
      <div className="a">Ordering exists only within one partition of one topic, so the partition key is
      the aggregate id &mdash; the payment id. That gives ordering per payment and parallelism across
      payments. The catch people miss: if your events live in separate topics per event type, there
      is no ordering between them no matter what the key is.</div>
    </div>

    <div className="qa">
      <div className="q">Why not just publish to Kafka directly?</div>
      <div className="a">A database commit and a publish are two systems with no shared transaction.
      Publish-then-commit announces payments that do not exist; commit-then-publish loses payments
      silently. The outbox makes both writes one transaction and lets a relay handle delivery, which
      is allowed to be unreliable because consumers deduplicate.</div>
    </div>

    <div className="qa">
      <div className="q">What would you add next?</div>
      <div className="a">Retention on the three append-only tables; leader election for the relay once
      there is more than one replica; a dead-letter drain; per-currency exponents if this stopped
      being IDR-only; and real observability &mdash; the counts by status are the business metric, and
      a rising <code>UNKNOWN</code> count is the page-someone signal.</div>
    </div>

    <div className="callout green" style={{marginTop: '24px'}}>
      <span className="lbl">If you remember one thing</span>
      <b>You cannot tell a timeout from a failure.</b> Every mechanism here &mdash; the outbox, the
      idempotency keys, the inbox, <code>UNKNOWN</code>, the reconciler &mdash; exists to make that
      ambiguity survivable rather than to pretend it away.
    </div>
  </section>


      </div>
    </>
  );
}
