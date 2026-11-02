export function Toast({ toast }) {
  return (
    <div className={`toast${toast ? ` show ${toast.kind}` : ''}`}>
      {toast?.message}
    </div>
  );
}
