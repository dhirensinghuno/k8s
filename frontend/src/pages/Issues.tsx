import { useState, useEffect } from 'react';
import { AlertTriangle, CheckCircle } from 'lucide-react';
const API_BASE = 'http://localhost:8888';

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
  evidence: string[];
  resolved: boolean;
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
            {issues.map(issue => (
              <tr key={issue.id}>
                <td>
                  <span className={`issue-tag ${getSeverityClass(issue.severity)}`}>
                    {issue.severity}
                  </span>
                </td>
                <td>{issue.type}</td>
                <td>{issue.namespace}</td>
                <td>{issue.pod}</td>
                <td>{issue.reason}</td>
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
            ))}
          </tbody>
        </table>
      </div>

      {selectedIssue && (
        <div className="modal-overlay" onClick={() => setSelectedIssue(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: 600 }}>
            <h3>Issue Details</h3>
            <div style={{ marginTop: '1rem' }}>
              <p><strong>Type:</strong> {selectedIssue.type}</p>
              <p><strong>Severity:</strong> {selectedIssue.severity}</p>
              <p><strong>Namespace:</strong> {selectedIssue.namespace}</p>
              <p><strong>Pod:</strong> {selectedIssue.pod}</p>
              <p><strong>Root Cause:</strong> {selectedIssue.root_cause}</p>
              <p><strong>Reason:</strong> {selectedIssue.reason}</p>
              <p><strong>Message:</strong> {selectedIssue.message}</p>
              {selectedIssue.evidence.length > 0 && (
                <div style={{ marginTop: '1rem' }}>
                  <strong>Evidence:</strong>
                  <ul style={{ marginTop: '0.5rem', paddingLeft: '1.5rem' }}>
                    {selectedIssue.evidence.map((e, idx) => (
                      <li key={idx}>{e}</li>
                    ))}
                  </ul>
                </div>
              )}
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
