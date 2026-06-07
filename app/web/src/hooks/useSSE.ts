import { useEffect, useRef, useState, useCallback } from 'react';
import { InverterStatus } from '@/types/status';
import { API_BASE } from '@/lib/api';

interface SSEHookReturn {
  status: InverterStatus | null;
  isConnected: boolean;
  error: string | null;
  reconnect: () => void;
}

export function useSSE(): SSEHookReturn {
  const [status, setStatus] = useState<InverterStatus | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cleanup = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    cleanup();

    try {
      const url = `${API_BASE}/events`;
      const eventSource = new EventSource(url);
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        setIsConnected(true);
        setError(null);
        console.log('SSE connection established');
      };

      eventSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as InverterStatus;
          setStatus(data);
        } catch (err) {
          console.error('Failed to parse SSE message:', err);
          setError('Failed to parse server data');
        }
      };

      eventSource.onerror = () => {
        console.error('SSE connection error');
        setIsConnected(false);

        if (eventSource.readyState === EventSource.CLOSED) {
          setError('Connection closed by server');
        } else {
          setError('Connection error');
        }

        reconnectTimeoutRef.current = setTimeout(() => {
          if (eventSourceRef.current === eventSource) {
            console.log('Attempting to reconnect SSE...');
            connect();
          }
        }, 3000);
      };

    } catch (err) {
      console.error('Failed to create SSE connection:', err);
      setError('Failed to connect to server');
    }
  }, [cleanup]);

  useEffect(() => {
    connect();
    return cleanup;
  }, [connect, cleanup]);

  const reconnect = useCallback(() => {
    console.log('Manual SSE reconnect requested');
    setError(null);
    connect();
  }, [connect]);

  return { status, isConnected, error, reconnect };
}
