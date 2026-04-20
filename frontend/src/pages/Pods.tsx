import { useState, useEffect } from 'react';
const API_BASE = '';

interface Pod {
  name: string;
  namespace: string;
  status: string;
  ready: boolean;
  restarts: number;
  image: string;
  node: string;
  issue_types: string[];
  reason: string;
}

export default function Pods() {
  const [pods, setPods] = useState<Pod[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('all');

  useEffect(() => {
    fetchPods();
    const interval = setInterval(fetchPods, 15000);
    return () => clearInterval(interval);
  }, []);

  const fetchPods = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/pods`);
      const data = await res.json();
      setPods(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch pods:', err);
      setLoading(false);
    }
  };

  const filteredPods = pods.filter(pod => {
    if (filter === 'all') return true;
    if (filter === 'unhealthy') return pod.issue_types.length > 0 || !pod.ready;
    if (filter === 'crashloop') return pod.issue_types.includes('CrashLoopBackOff');
    return true;
  });

  const getIssueBadgeClass = (type: string) => {
    switch (type) {
      case 'CrashLoopBackOff':
      case 'OOMKilled':
      case 'ImagePullBackOff':
        return 'issue-critical';
      default:
        return 'issue-warning';
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Pods ({filteredPods.length})</h2>
        <select 
          value={filter} 
          onChange={(e) => setFilter(e.target.value)}
          style={{ padding: '0.5rem', borderRadius: '0.5rem', background: '#1e293b', color: '#fff', border: '1px solid #334155' }}
        >
          <option value="all">All Pods</option>
          <option value="unhealthy">Unhealthy</option>
          <option value="crashloop">CrashLoopBackOff</option>
        </select>
      </div>

      <div className="table-container">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Namespace</th>
              <th>Status</th>
              <th>Ready</th>
              <th>Restarts</th>
              <th>Issues</th>
              <th>Node</th>
            </tr>
          </thead>
          <tbody>
            {filteredPods.map(pod => (
              <tr key={`${pod.namespace}-${pod.name}`}>
                <td>{pod.name}</td>
                <td>{pod.namespace}</td>
                <td>{pod.status}</td>
                <td style={{ color: pod.ready ? '#22c55e' : '#ef4444' }}>
                  {pod.ready ? 'Yes' : 'No'}
                </td>
                <td style={{ color: pod.restarts > 5 ? '#f59e0b' : 'inherit' }}>
                  {pod.restarts}
                </td>
                <td>
                  {pod.issue_types.map((type, idx) => (
                    <span key={idx} className={`issue-tag ${getIssueBadgeClass(type)}`}>
                      {type}
                    </span>
                  ))}
                </td>
                <td>{pod.node}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {filteredPods.length === 0 && (
        <p style={{ color: '#64748b', textAlign: 'center', padding: '2rem' }}>
          No pods match the filter
        </p>
      )}
    </div>
  );
}
