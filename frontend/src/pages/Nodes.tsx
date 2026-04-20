import { useState, useEffect } from 'react';
const API_BASE = '';

interface Node {
  name: string;
  status: string;
  ready: boolean;
  conditions: string[];
}

export default function Nodes() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchNodes();
    const interval = setInterval(fetchNodes, 15000);
    return () => clearInterval(interval);
  }, []);

  const fetchNodes = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/nodes`);
      const data = await res.json();
      setNodes(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch nodes:', err);
      setLoading(false);
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Nodes ({nodes.length})</h2>
      </div>

      <div className="nodes-grid">
        {nodes.map(node => (
          <div key={node.name} className="node-card">
            <div className="node-card-header">
              <h3>{node.name}</h3>
              <span className={`status-badge ${node.ready ? 'status-healthy' : 'status-critical'}`}>
                {node.status}
              </span>
            </div>
            {node.conditions.length > 0 ? (
              <div>
                <h4 style={{ fontSize: '0.875rem', color: '#94a3b8', marginBottom: '0.5rem' }}>Conditions:</h4>
                {node.conditions.map((cond, idx) => (
                  <span key={idx} className="issue-tag issue-warning">
                    {cond}
                  </span>
                ))}
              </div>
            ) : (
              <p style={{ color: '#64748b' }}>All conditions normal</p>
            )}
          </div>
        ))}
      </div>

      {nodes.length === 0 && (
        <p style={{ color: '#64748b', textAlign: 'center', padding: '2rem' }}>
          No nodes found
        </p>
      )}
    </div>
  );
}
