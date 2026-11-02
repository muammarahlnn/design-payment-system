export function Card({ title, hint, actions, children, bodyless = false }) {
  return (
    <div className="card">
      {(title || actions) && (
        <header>
          {title && <h2>{title}</h2>}
          {hint && <span className="hint">{hint}</span>}
          {actions && (
            <>
              <div className="spacer" />
              {actions}
            </>
          )}
        </header>
      )}
      {bodyless ? children : <div className="body">{children}</div>}
    </div>
  );
}
