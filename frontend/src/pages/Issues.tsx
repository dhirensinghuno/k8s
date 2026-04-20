import { useState, useEffect } from 'react';
import { AlertTriangle, CheckCircle } from 'lucide-react';
const API_BASE = '';

interface Issue {
  id: string;
  timestamp: string;
  severity: string;
  type: string;
  namespace: string;
  pod: string;
  node: string;
  deployment: string;
  reason: string;
  message: string;
  root_cause: string;
  evidence: string[] | null;
  resolved: boolean;
  resolved_at: string | null;
}

export default function Issues() {
  const [issues, setIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);

  useEffect(() => {
    fetchIssues();
    const interval = setInterval(fetchIssues, 15000);
    return () => clearInterval(interval);
  }, []);

  const fetchIssues = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/issues`);
      const data = await res.json();
      setIssues(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch issues:', err);
      setLoading(false);
    }
  };

  const resolveIssue = async (id: string) => {
    try {
      await fetch(`${API_BASE}/api/issues/${id}/resolve`, { method: 'POST' });
      fetchIssues();
    } catch (err) {
      console.error('Failed to resolve issue:', err);
    }
  };

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  const getSeverityClass = (severity: string) => {
    return severity === 'critical' ? 'issue-critical' : 'issue-warning';
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Issues ({issues.length})</h2>
      </div>

      <div className="table-container">
        <table>
          <thead>
            <tr>
              <th>Severity</th>
              <th>Type</th>
              <th>Namespace</th>
              <th>Pod</th>
              <th>Reason</th>
              <th>Time</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {issues.length === 0 ? (
              <tr>
                <td colSpan={8} style={{ textAlign: 'center', color: '#64748b' }}>
                  No issues detected
                </td>
              </tr>
            ) : (
              issues.map(issue => (
                <tr key={issue.id}>
                  <td>
                    <span className={`issue-tag ${getSeverityClass(issue.severity)}`}>
                      {issue.severity}
                    </span>
                  </td>
                  <td>{issue.type}</td>
                  <td>{issue.namespace}</td>
                  <td>{issue.pod}</td>
                  <td>{issue.reason || '-'}</td>
                  <td>{formatTime(issue.timestamp)}</td>
                  <td style={{ color: issue.resolved ? '#22c55e' : '#f59e0b' }}>
                    {issue.resolved ? 'Resolved' : 'Active'}
                  </td>
                  <td>
                    <button 
                      className="btn btn-secondary" 
                      style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                      onClick={() => setSelectedIssue(issue)}
                    >
                      Details
                    </button>
                    {!issue.resolved && (
                      <button 
                        className="btn btn-primary" 
                        style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem', marginLeft: '0.5rem' }}
                        onClick={() => resolveIssue(issue.id)}
                      >
                        <CheckCircle size={14} /> Resolve
                      </button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {selectedIssue && (
        <div className="modal-overlay" onClick={() => setSelectedIssue(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: 600 }}>
            <h3>Issue Details</h3>
            <div style={{ marginTop: '1rem' }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>ID</label>
                  <p style={{ fontWeight: 500 }}>{selectedIssue.id}</p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Severity</label>
                  <p style={{ fontWeight: 500, color: selectedIssue.severity === 'critical' ? '#ef4444' : '#f59e0b' }}>
                    {selectedIssue.severity?.toUpperCase() || 'N/A'}
                  </p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Type</label>
                  <p style={{ fontWeight: 500 }}>{selectedIssue.type || 'N/A'}</p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Status</label>
                  <p style={{ fontWeight: 500, color: selectedIssue.resolved ? '#22c55e' : '#f59e0b' }}>
                    {selectedIssue.resolved ? 'RESOLVED' : 'ACTIVE'}
                  </p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Namespace</label>
                  <p style={{ fontWeight: 500 }}>{selectedIssue.namespace || 'N/A'}</p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Pod</label>
                  <p style={{ fontWeight: 500 }}>{selectedIssue.pod || 'N/A'}</p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Node</label>
                  <p style={{ fontWeight: 500 }}>{selectedIssue.node || 'N/A'}</p>
                </div>
                <div>
                  <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Deployment</label>
                  <p style={{ fontWeight: 500 }}>{selectedIssue.deployment || 'N/A'}</p>
                </div>
              </div>
              <div style={{ marginTop: '1rem' }}>
                <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Reason</label>
                <p>{selectedIssue.reason || 'Unknown'}</p>
              </div>
              <div style={{ marginTop: '1rem' }}>
                <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Message</label>
                <p>{selectedIssue.message || 'No message available'}</p>
              </div>
              <div style={{ marginTop: '1rem' }}>
                <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Root Cause</label>
                <p>{selectedIssue.root_cause || 'Not determined'}</p>
              </div>
              <div style={{ marginTop: '1rem' }}>
                <label style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Timestamp</label>
                <p>{formatTime(selectedIssue.timestamp)}</p>
              </div>
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setSelectedIssue(null)}>Close</button>
              {!selectedIssue.resolved && (
                <button className="btn btn-primary" onClick={() => { resolveIssue(selectedIssue.id); setSelectedIssue(null); }}>
                  Mark Resolved
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
