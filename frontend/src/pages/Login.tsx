import { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { useNavigate } from 'react-router-dom';
import { LogIn } from 'lucide-react';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await login(username, password);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: '#0f172a',
    }}>
      <div style={{
        background: '#1e293b',
        padding: '2rem',
        borderRadius: '0.5rem',
        width: '100%',
        maxWidth: '400px',
        boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: '1.5rem' }}>
          <LogIn size={32} style={{ marginRight: '0.5rem', color: '#3b82f6' }} />
          <h1 style={{ fontSize: '1.5rem', fontWeight: 'bold', color: '#fff' }}>K8s SRE Login</h1>
        </div>

        <form onSubmit={handleSubmit}>
          {error && (
            <div style={{
              background: '#7f1d1d',
              color: '#fecaca',
              padding: '0.75rem',
              borderRadius: '0.25rem',
              marginBottom: '1rem',
              fontSize: '0.875rem',
            }}>
              {error}
            </div>
          )}

          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', color: '#94a3b8', fontSize: '0.875rem' }}>
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              style={{
                width: '100%',
                padding: '0.75rem',
                background: '#334155',
                border: '1px solid #475569',
                borderRadius: '0.25rem',
                color: '#fff',
                fontSize: '1rem',
              }}
            />
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', color: '#94a3b8', fontSize: '0.875rem' }}>
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              style={{
                width: '100%',
                padding: '0.75rem',
                background: '#334155',
                border: '1px solid #475569',
                borderRadius: '0.25rem',
                color: '#fff',
                fontSize: '1rem',
              }}
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            style={{
              width: '100%',
              padding: '0.75rem',
              background: '#3b82f6',
              color: '#fff',
              border: 'none',
              borderRadius: '0.25rem',
              fontSize: '1rem',
              fontWeight: '500',
              cursor: loading ? 'not-allowed' : 'pointer',
              opacity: loading ? 0.7 : 1,
            }}
          >
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>

        <div style={{ marginTop: '1.5rem', textAlign: 'center', color: '#64748b', fontSize: '0.875rem' }}>
          <p>Default admin credentials:</p>
          <p style={{ color: '#94a3b8' }}>Username: admin | Password: (set via --auth-admin-password)</p>
        </div>
      </div>
    </div>
  );
}