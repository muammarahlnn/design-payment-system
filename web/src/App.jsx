import { Navigate, Route, Routes } from 'react-router-dom';
import { Playground } from './pages/Playground';
import { Learn } from './pages/Learn';

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Playground />} />
      <Route path="/learn" element={<Learn />} />
      {/* The vanilla build linked to /learn.html; keep old links working. */}
      <Route path="/learn.html" element={<Navigate to="/learn" replace />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
