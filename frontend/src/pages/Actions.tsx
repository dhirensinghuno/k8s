import { useState, useEffect } from 'react';
import { RotateCcw, Wrench, AlertCircle } from 'lucide-react';
const API_BASE = '';

interface Action {
  id: string;
  timestamp: string;
  issue_id: string;
  type: string;
  target: string;
  namespace: string;
  reason: string;
  success: boolean;
  result: string;
  rollback_from: boolean;
  prev_version: string;
  new_version: string;
}

export default function Actions() {
  const [actions, setActions] = useState<Action[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchActions();
    const interval = setInterval(fetchActions, 15000);
    return () => clearInterval(interval);
  }, []);

  const fetchActions = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/actions`);
      const data = await res.json();
      setActions(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch actions:', err);
      setLoading(false);
    }
  };

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  const getActionIcon = (type: string) => {
    switch (type) {
      case 'Rollback': return <RotateCcw size={16} />;
      case 'RestartPod': return <Wrench size={16} />;
      default: return <AlertCircle size={16} />;
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Remediation Actions ({actions.length})</h2>
      </div>

      <div className="table-container">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Type</th>
              <th>Target</th>
              <th>Namespace</th>
              <th>Reason</th>
              <th>Result</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {actions.map(action => (
              <tr key={action.id}>
                <td>{formatTime(action.timestamp)}</td>
                <td style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  {getActionIcon(action.type)}
                  {action.type}
                  {action.rollback_from && (
                    <span style={{ fontSize: '0.75rem', color: '#f59e0b' }}>Rollback</span>
                  )}
                </td>
                <td>{action.target}</td>
                <td>{action.namespace}</td>
                <td style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {action.reason}
                </td>
                <td style={{ fontSize: '0.75rem', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {action.result}
                </td>
                <td style={{ color: action.success ? '#22c55e' : '#ef4444' }}>
                  {action.success ? 'Success' : 'Failed'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {actions.length === 0 && (
        <p style={{ color: '#64748b', textAlign: 'center', padding: '2rem' }}>
          No remediation actions recorded
        </p>
      )}
    </div>
  );
}
