import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router-dom';
import { LayoutDashboard, Server, Container, AlertTriangle, Activity, GitBranch, ScrollText, LogOut } from 'lucide-react';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import Dashboard from './pages/Dashboard';
import Nodes from './pages/Nodes';
import Pods from './pages/Pods';
import Events from './pages/Events';
import Issues from './pages/Issues';
import Actions from './pages/Actions';
import Deployments from './pages/Deployments';
import Audit from './pages/Audit';
import Login from './pages/Login';

const API_BASE = '';

export { API_BASE };

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  
  if (isLoading) {
    return (
      <div style={{ padding: '2rem', textAlign: 'center' }}>
        <p>Loading...</p>
      </div>
    );
  }
  
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  
  return <>{children}</>;
}

function DashboardLayout() {
  const [health, setHealth] = useState<any>(null);
  const { user, logout, isAuthenticated } = useAuth();

  useEffect(() => {
    fetchHealth();
    const interval = setInterval(fetchHealth, 10000);
    return () => clearInterval(interval);
  }, []);

  const fetchHealth = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/health`);
      const data = await res.json();
      setHealth(data);
    } catch (err) {
      console.error('Failed to fetch health:', err);
    }
  };

  const getStatusClass = () => {
    if (!health) return '';
    switch (health.overall_status) {
      case 'healthy': return 'status-healthy';
      case 'warning': return 'status-warning';
      case 'critical': return 'status-critical';
      default: return '';
    }
  };

  return (
    <div className="dashboard">
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1>K8s SRE Agent</h1>
        </div>
        <nav className="nav-links">
          <NavLink to="/" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <LayoutDashboard size={20} />
            Dashboard
          </NavLink>
          <NavLink to="/nodes" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <Server size={20} />
            Nodes
          </NavLink>
          <NavLink to="/pods" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <Container size={20} />
            Pods
          </NavLink>
          <NavLink to="/deployments" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <GitBranch size={20} />
            Deployments
          </NavLink>
          <NavLink to="/issues" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <AlertTriangle size={20} />
            Issues
          </NavLink>
          <NavLink to="/events" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <Activity size={20} />
            Events
          </NavLink>
          <NavLink to="/actions" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <ScrollText size={20} />
            Actions
          </NavLink>
          <NavLink to="/audit" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <ScrollText size={20} />
            Audit Log
          </NavLink>
        </nav>
        {health && (
          <div style={{ marginTop: 'auto', padding: '1rem', borderTop: '1px solid #334155' }}>
            <div className={`status-badge ${getStatusClass()}`}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'currentColor' }}></span>
              {health.overall_status?.toUpperCase() || 'UNKNOWN'}
            </div>
          </div>
        )}
        {user && (
          <div style={{ padding: '1rem', borderTop: '1px solid #334155' }}>
            <div style={{ marginBottom: '0.5rem', fontSize: '0.875rem', color: '#94a3b8' }}>
              Logged in as: <strong>{user.username}</strong>
            </div>
            <div style={{ fontSize: '0.75rem', color: '#64748b', marginBottom: '0.5rem' }}>
              Role: {user.role}
            </div>
            <button
              onClick={logout}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.5rem',
                background: 'transparent',
                border: '1px solid #475569',
                color: '#94a3b8',
                padding: '0.5rem',
                borderRadius: '0.25rem',
                cursor: 'pointer',
                fontSize: '0.875rem',
                width: '100%',
              }}
            >
              <LogOut size={16} />
              Logout
            </button>
          </div>
        )}
      </aside>
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/nodes" element={<Nodes />} />
          <Route path="/pods" element={<Pods />} />
          <Route path="/deployments" element={<Deployments />} />
          <Route path="/issues" element={<Issues />} />
          <Route path="/events" element={<Events />} />
          <Route path="/actions" element={<Actions />} />
          <Route path="/audit" element={<Audit />} />
        </Routes>
      </main>
    </div>
  );
}

function AppRoutes() {
  const { isAuthenticated } = useAuth();
  
  return (
    <Routes>
      <Route path="/login" element={isAuthenticated ? <Navigate to="/" replace /> : <Login />} />
      <Route path="/*" element={
        <ProtectedRoute>
          <DashboardLayout />
        </ProtectedRoute>
      } />
    </Routes>
  );
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
