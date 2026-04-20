import { useState, useEffect } from 'react';
import { GitBranch, RotateCcw, RefreshCw } from 'lucide-react';
const API_BASE = '';

interface Deployment {
  name: string;
  namespace: string;
  replicas: number;
  ready_replicas: number;
  updated_replicas: number;
  image: string;
}

export default function Deployments() {
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(true);
  const [rollbackConfirm, setRollbackConfirm] = useState<{namespace: string; name: string} | null>(null);

  useEffect(() => {
    fetchDeployments();
    const interval = setInterval(fetchDeployments, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchDeployments = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/deployments`);
      const data = await res.json();
      setDeployments(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch deployments:', err);
      setLoading(false);
    }
  };

  const handleRollback = async (namespace: string, name: string) => {
    try {
      const url = `${API_BASE}/api/deployments/${namespace}/${name}/rollback`;
      console.log('[Deployments] Rollback URL:', url);
      console.log('[Deployments] API_BASE:', API_BASE);
      
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason: 'Manual rollback via dashboard' })
      });
      
      console.log('[Deployments] Rollback response status:', res.status);
      console.log('[Deployments] Rollback response ok:', res.ok);
      const text = await res.text();
      console.log('[Deployments] Rollback response text:', text);
      
      if (!res.ok) {
        alert(`Rollback failed: HTTP ${res.status} - ${text}`);
        return;
      }
      
      let data;
      try {
        data = JSON.parse(text);
        alert(`Rollback successful: ${data.message || text}`);
      } catch {
        alert(`Rollback successful: ${text}`);
      }
      setRollbackConfirm(null);
      fetchDeployments();
    } catch (err) {
      console.error('Rollback failed:', err);
      const errMsg = err instanceof Error ? err.message : String(err);
      alert(`Rollback failed: ${errMsg}`);
    }
  };

  const handleRestart = async (namespace: string, name: string) => {
    try {
      const url = `${API_BASE}/api/deployments/${namespace}/${name}/restart`;
      console.log('[Deployments] Restart URL:', url);
      console.log('[Deployments] API_BASE:', API_BASE);
      
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      
      console.log('[Deployments] Restart response status:', res.status);
      console.log('[Deployments] Restart response ok:', res.ok);
      const text = await res.text();
      console.log('[Deployments] Restart response text:', text);
      
      if (!res.ok) {
        alert(`Restart failed: HTTP ${res.status} - ${text}`);
        return;
      }
      
      let data;
      try {
        data = JSON.parse(text);
        alert(`Restart successful: ${data.message || text}`);
      } catch {
        alert(`Restart successful: ${text}`);
      }
      fetchDeployments();
    } catch (err) {
      console.error('Restart failed:', err);
      const errMsg = err instanceof Error ? err.message : String(err);
      alert(`Restart failed: ${errMsg}`);
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Deployments ({deployments.length})</h2>
      </div>

      <div className="table-container">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Namespace</th>
              <th>Replicas</th>
              <th>Ready</th>
              <th>Updated</th>
              <th>Image</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {deployments.map(dep => (
              <tr key={`${dep.namespace}-${dep.name}`}>
                <td>{dep.name}</td>
                <td>{dep.namespace}</td>
                <td>{dep.replicas}</td>
                <td style={{ color: dep.ready_replicas === dep.replicas ? '#22c55e' : '#f59e0b' }}>
                  {dep.ready_replicas}/{dep.replicas}
                </td>
                <td>{dep.updated_replicas}</td>
                <td style={{ fontSize: '0.75rem', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {dep.image}
                </td>
                <td>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <button 
                      className="btn btn-secondary" 
                      style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                      onClick={() => handleRestart(dep.namespace, dep.name)}
                    >
                      <RefreshCw size={14} /> Restart
                    </button>
                    <button 
                      className="btn btn-danger" 
                      style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                      onClick={() => setRollbackConfirm({ namespace: dep.namespace, name: dep.name })}
                    >
                      <RotateCcw size={14} /> Rollback
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {rollbackConfirm && (
        <div className="modal-overlay" onClick={() => setRollbackConfirm(null)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h3>Confirm Rollback</h3>
            <p>Are you sure you want to rollback <strong>{rollbackConfirm.name}</strong> in namespace <strong>{rollbackConfirm.namespace}</strong>?</p>
            <p style={{ color: '#f59e0b', marginTop: '1rem' }}>This will restart all pods with the previous image.</p>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setRollbackConfirm(null)}>Cancel</button>
              <button className="btn btn-danger" onClick={() => handleRollback(rollbackConfirm.namespace, rollbackConfirm.name)}>
                Confirm Rollback
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
