import { useState, useEffect } from 'react';
const API_BASE = '';

interface AuditLog {
  id: number;
  timestamp: string;
  level: string;
  component: string;
  message: string;
  issue_id: string;
  action_id: string;
}

export default function Audit() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchLogs();
    const interval = setInterval(fetchLogs, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchLogs = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/audit`);
      const data = await res.json();
      setLogs(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch audit logs:', err);
      setLoading(false);
    }
  };

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  const getLevelColor = (level: string) => {
    switch (level) {
      case 'info': return '#38bdf8';
      case 'warn': return '#f59e0b';
      case 'error': return '#ef4444';
      default: return '#94a3b8';
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Audit Log ({logs.length})</h2>
        <button className="btn btn-secondary" onClick={fetchLogs}>Refresh</button>
      </div>

      <div className="logs-container">
        {logs.length === 0 ? (
          <p style={{ color: '#64748b' }}>No audit logs available</p>
        ) : (
          logs.map(log => (
            <div key={log.id} style={{ marginBottom: '0.5rem', padding: '0.5rem', borderBottom: '1px solid #1e293b' }}>
              <span style={{ color: '#64748b' }}>[{formatTime(log.timestamp)}]</span>{' '}
              <span style={{ color: getLevelColor(log.level), fontWeight: 'bold' }}>[{log.level.toUpperCase()}]</span>{' '}
              <span style={{ color: '#38bdf8' }}>[{log.component}]</span>{' '}
              <span>{log.message}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
