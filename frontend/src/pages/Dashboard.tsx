import { useState, useEffect } from 'react';
import { Server, Container, AlertTriangle, CheckCircle, Activity } from 'lucide-react';
const API_BASE = 'http://localhost:8888';

interface Health {
  overall_status: string;
  nodes_ready: number;
  nodes_total: number;
  pods_running: number;
  pods_total: number;
  pods_unhealthy: number;
  critical_issues: number;
  warning_issues: number;
  warning_events: number;
}

export default function Dashboard() {
  const [health, setHealth] = useState<Health | null>(null);
  const [recentEvents, setRecentEvents] = useState<any[]>([]);
  const [recentActions, setRecentActions] = useState<any[]>([]);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, []);

  const fetchData = async () => {
    try {
      const [healthRes, eventsRes, actionsRes] = await Promise.all([
        fetch(`${API_BASE}/api/health`),
        fetch(`${API_BASE}/api/events`),
        fetch(`${API_BASE}/api/actions`)
      ]);
      const [h, e, a] = await Promise.all([healthRes.json(), eventsRes.json(), actionsRes.json()]);
      setHealth(h);
      setRecentEvents(e.slice(0, 5));
      setRecentActions(a.slice(0, 5));
    } catch (err) {
      console.error('Failed to fetch data:', err);
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

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleTimeString();
  };

  return (
    <div>
      <div className="header">
        <h2>Cluster Overview</h2>
        {health && (
          <div className={`status-badge ${getStatusClass()}`}>
            <CheckCircle size={16} />
            {health.overall_status?.toUpperCase() || 'UNKNOWN'}
          </div>
        )}
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <h3><Server size={16} style={{ marginRight: 8 }} />Nodes</h3>
          <div className="value">
            {health?.nodes_ready || 0}/{health?.nodes_total || 0}
          </div>
          <div className="sub">Ready</div>
        </div>

        <div className="stat-card">
          <h3><Container size={16} style={{ marginRight: 8 }} />Pods</h3>
          <div className="value">
            {health?.pods_running || 0}/{health?.pods_total || 0}
          </div>
          <div className="sub">Running</div>
        </div>

        <div className="stat-card">
          <h3><AlertTriangle size={16} style={{ marginRight: 8, color: '#ef4444' }} />Critical Issues</h3>
          <div className="value" style={{ color: health?.critical_issues ? '#ef4444' : '#22c55e' }}>
            {health?.critical_issues || 0}
          </div>
          <div className="sub">Require attention</div>
        </div>

        <div className="stat-card">
          <h3><AlertTriangle size={16} style={{ marginRight: 8, color: '#f59e0b' }} />Warning Issues</h3>
          <div className="value" style={{ color: health?.warning_issues ? '#f59e0b' : '#22c55e' }}>
            {health?.warning_issues || 0}
          </div>
          <div className="sub">Monitor closely</div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2rem' }}>
        <div>
          <h3 style={{ marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Activity size={20} /> Recent Events
          </h3>
          {recentEvents.length === 0 ? (
            <p style={{ color: '#64748b' }}>No recent events</p>
          ) : (
            recentEvents.map((event, idx) => (
              <div key={idx} className={`event-item event-${event.type?.toLowerCase() || 'warning'}`}>
                <div className="event-header">
                  <span className="event-reason">{event.reason}</span>
                  <span className="event-time">{formatTime(event.last_seen)}</span>
                </div>
                <div className="event-message">{event.message}</div>
              </div>
            ))
          )}
        </div>

        <div>
          <h3 style={{ marginBottom: '1rem' }}>Recent Actions</h3>
          {recentActions.length === 0 ? (
            <p style={{ color: '#64748b' }}>No recent actions</p>
          ) : (
            <div className="table-container">
              <table>
                <thead>
                  <tr>
                    <th>Type</th>
                    <th>Target</th>
                    <th>Result</th>
                    <th>Time</th>
                  </tr>
                </thead>
                <tbody>
                  {recentActions.map((action: any) => (
                    <tr key={action.id}>
                      <td>{action.type}</td>
                      <td>{action.target}</td>
                      <td style={{ color: action.success ? '#22c55e' : '#ef4444' }}>
                        {action.success ? 'Success' : 'Failed'}
                      </td>
                      <td>{formatTime(action.timestamp)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
