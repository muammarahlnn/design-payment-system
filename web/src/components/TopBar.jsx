import { NavLink } from 'react-router-dom';
import { useHealth } from '../hooks/useHealth';

export function TopBar({ subtitle }) {
  const services = useHealth();

  return (
    <header className="topbar">
      <div className="mark">S</div>
      <div>
        <h1>Payment System</h1>
        <div className="sub">{subtitle}</div>
      </div>
      <div className="spacer" />
      <nav className="nav" style={{ marginRight: 12 }}>
        <NavLink to="/" className={({ isActive }) => (isActive ? 'on' : '')} end>
          Playground
        </NavLink>
        <NavLink to="/learn" className={({ isActive }) => (isActive ? 'on' : '')}>
          How it works
        </NavLink>
      </nav>
      <div className="health">
        {services.map(({ name, up }) => (
          <span key={name} className={`dot ${up === null ? '' : up ? 'up' : 'down'}`}>
            <i />
            {name}
          </span>
        ))}
      </div>
    </header>
  );
}
