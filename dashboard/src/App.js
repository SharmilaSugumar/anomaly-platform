import { useState, useEffect } from 'react';

export default function App() {
  const [alerts, setAlerts] = useState([]);
  const [connected, setConnected] = useState(false);
  const [stats, setStats] = useState({ total: 0, maxSpeed: 0, avgSpeed: 0 });

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/ws');
    ws.onopen = () => setConnected(true);
    ws.onmessage = (event) => {
      const alert = JSON.parse(event.data);
      setAlerts(prev => {
        const next = [alert, ...prev].slice(0, 50);
        const speeds = next.map(a => a.SpeedKMH).filter(Boolean);
        setStats({
          total: next.length,
          maxSpeed: Math.max(...speeds).toFixed(1),
          avgSpeed: (speeds.reduce((a, b) => a + b, 0) / speeds.length).toFixed(1)
        });
        return next;
      });
    };
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);
    return () => ws.close();
  }, []);

  // Also fetch history from REST on load
  useEffect(() => {
    fetch('http://localhost:8080/api/alerts')
      .then(r => r.json())
      .then(data => {
        if (data.alerts) setAlerts(data.alerts.slice(0, 50));
      })
      .catch(() => {});
  }, []);

  const styles = {
    app: {
      fontFamily: "'Courier New', monospace",
      background: '#07080d',
      minHeight: '100vh',
      color: '#8899bb',
      padding: '0',
    },
    header: {
      background: '#0c0e16',
      borderBottom: '1px solid #1e2235',
      padding: '16px 32px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
    },
    title: {
      fontSize: '18px',
      fontWeight: 'bold',
      color: '#dde8ff',
      margin: 0,
    },
    status: {
      fontSize: '11px',
      letterSpacing: '2px',
      textTransform: 'uppercase',
      color: connected ? '#5bf5d0' : '#f87171',
      display: 'flex',
      alignItems: 'center',
      gap: '8px',
    },
    dot: {
      width: '8px',
      height: '8px',
      borderRadius: '50%',
      background: connected ? '#5bf5d0' : '#f87171',
      boxShadow: connected ? '0 0 8px #5bf5d0' : '0 0 8px #f87171',
    },
    body: { padding: '24px 32px' },
    statsRow: {
      display: 'grid',
      gridTemplateColumns: 'repeat(3, 1fr)',
      gap: '16px',
      marginBottom: '24px',
    },
    statCard: {
      background: '#0c0e16',
      border: '1px solid #1e2235',
      borderRadius: '8px',
      padding: '20px',
    },
    statLabel: {
      fontSize: '9px',
      letterSpacing: '3px',
      textTransform: 'uppercase',
      color: '#3a4560',
      marginBottom: '8px',
    },
    statValue: {
      fontSize: '28px',
      fontWeight: 'bold',
      color: '#dde8ff',
    },
    tableHeader: {
      fontSize: '9px',
      letterSpacing: '3px',
      textTransform: 'uppercase',
      color: '#3a4560',
      marginBottom: '12px',
    },
    alertRow: {
      background: '#0c0e16',
      border: '1px solid #1e2235',
      borderLeft: '3px solid #f87171',
      borderRadius: '6px',
      padding: '12px 16px',
      marginBottom: '6px',
      display: 'grid',
      gridTemplateColumns: '120px 100px 120px 1fr 100px',
      alignItems: 'center',
      gap: '16px',
      fontSize: '11px',
    },
    alertType: { color: '#f87171', fontWeight: 'bold' },
    trackId: { color: '#fbbf24' },
    speed: { color: '#dde8ff', fontWeight: 'bold' },
    zone: { color: '#5bf5d0' },
    time: { color: '#3a4560', fontSize: '10px', textAlign: 'right' },
  };

  return (
    <div style={styles.app}>
      <div style={styles.header}>
        <h1 style={styles.title}>⚡ AI Anomaly Detection Pipeline</h1>
        <div style={styles.status}>
          <div style={styles.dot}></div>
          {connected ? 'Pipeline Live' : 'Disconnected'}
        </div>
      </div>

      <div style={styles.body}>
        {/* Stats */}
        <div style={styles.statsRow}>
          <div style={styles.statCard}>
            <div style={styles.statLabel}>Total Alerts</div>
            <div style={{...styles.statValue, color: '#f87171'}}>{stats.total}</div>
          </div>
          <div style={styles.statCard}>
            <div style={styles.statLabel}>Max Speed</div>
            <div style={{...styles.statValue, color: '#fbbf24'}}>{stats.maxSpeed} <span style={{fontSize:'14px'}}>km/h</span></div>
          </div>
          <div style={styles.statCard}>
            <div style={styles.statLabel}>Avg Speed</div>
            <div style={{...styles.statValue, color: '#5bf5d0'}}>{stats.avgSpeed} <span style={{fontSize:'14px'}}>km/h</span></div>
          </div>
        </div>

        {/* Alert table header */}
        <div style={styles.tableHeader}>// Live Alert Feed — Last {alerts.length} Events</div>

        {/* Alert rows */}
        {alerts.length === 0 && (
          <div style={{color: '#3a4560', fontSize: '12px', padding: '20px 0'}}>
            Waiting for alerts... ensure pipeline is running.
          </div>
        )}
        {alerts.map((a, i) => (
          <div key={i} style={styles.alertRow}>
            <span style={styles.alertType}>🚨 {a.AlertType}</span>
            <span style={styles.trackId}>Track #{a.TrackID}</span>
            <span style={styles.speed}>{a.SpeedKMH?.toFixed(1)} km/h</span>
            <span style={styles.zone}>{a.ZoneName} · {a.CamID}</span>
            <span style={styles.time}>
              {a.Timestamp ? new Date(a.Timestamp).toLocaleTimeString() : '—'}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}