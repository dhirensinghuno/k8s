import { useState, useEffect } from 'react';
const API_BASE = '';

interface Event {
  type: string;
  reason: string;
  message: string;
  involved: string;
  namespace: string;
  last_seen: string;
  count: number;
}

export default function Events() {
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchEvents();
    const interval = setInterval(fetchEvents, 15000);
    return () => clearInterval(interval);
  }, []);

  const fetchEvents = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/events`);
      const data = await res.json();
      setEvents(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (err) {
      console.error('Failed to fetch events:', err);
      setLoading(false);
    }
  };

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  const getEventClass = (type: string) => {
    return type === 'Warning' ? 'event-warning' : 'event-error';
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="header">
        <h2>Warning Events ({events.length})</h2>
      </div>

      {events.length === 0 ? (
        <p style={{ color: '#64748b', textAlign: 'center', padding: '2rem' }}>
          No warning events
        </p>
      ) : (
        events.map((event, idx) => (
          <div key={idx} className={`event-item ${getEventClass(event.type)}`}>
            <div className="event-header">
              <span className="event-reason">
                [{event.type}] {event.reason}
              </span>
              <span className="event-time">
                {formatTime(event.last_seen)} {event.count > 1 && `(x${event.count})`}
              </span>
            </div>
            <div className="event-message">{event.message}</div>
            <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: '0.5rem' }}>
              Namespace: {event.namespace} | Object: {event.involved}
            </div>
          </div>
        ))
      )}
    </div>
  );
}
