import axios from 'axios';
import type { Endpoint, WebhookRequest, ReplayResult, EndpointResponseConfig, AuthResponse } from './types';
import { getStoredToken } from './auth';

const http = axios.create({ baseURL: '/' });

// Attach JWT to every request when available.
http.interceptors.request.use(config => {
  const token = getStoredToken();
  if (token) {
    config.headers = config.headers ?? {};
    config.headers['Authorization'] = `Bearer ${token}`;
  }
  return config;
});

export const api = {
  auth: {
    register: async (name: string, email: string, password: string): Promise<AuthResponse> => {
      const res = await http.post<AuthResponse>('/auth/register', { name, email, password });
      return res.data;
    },
    login: async (email: string, password: string): Promise<AuthResponse> => {
      const res = await http.post<AuthResponse>('/auth/login', { email, password });
      return res.data;
    },
    me: async (): Promise<AuthResponse['user']> => {
      const res = await http.get<AuthResponse['user']>('/auth/me');
      return res.data;
    },
  },

  endpoints: {
    list: async (): Promise<Endpoint[]> => {
      const res = await http.get<Endpoint[]>('/api/endpoints');
      return res.data;
    },
    get: async (id: string): Promise<Endpoint> => {
      const res = await http.get<Endpoint>(`/api/endpoints/${id}`);
      return res.data;
    },
    create: async (targetUrl?: string, ttl?: string): Promise<Endpoint> => {
      const res = await http.post<Endpoint>('/api/endpoints', { target_url: targetUrl ?? '', ttl: ttl ?? '24h' });
      return res.data;
    },
    deleteAllRequests: async (id: string): Promise<void> => {
      await http.delete(`/api/endpoints/${id}/requests`);
    },
    delete: async (id: string): Promise<void> => {
      await http.delete(`/api/endpoints/${id}`);
    },
    updateResponse: async (id: string, config: EndpointResponseConfig): Promise<void> => {
      await http.patch(`/api/endpoints/${id}/response`, config);
    },
    rename: async (id: string, name: string): Promise<void> => {
      await http.patch(`/api/endpoints/${id}/name`, { name });
    },
    updateNotify: async (id: string, email: string): Promise<void> => {
      await http.patch(`/api/endpoints/${id}/notify`, { email });
    },
  },

  requests: {
    list: async (endpointId: string, limit = 200, offset = 0): Promise<WebhookRequest[]> => {
      const res = await http.get<WebhookRequest[]>(
        `/api/endpoints/${endpointId}/requests`,
        { params: { limit, offset } },
      );
      return res.data;
    },
    get: async (requestId: string): Promise<WebhookRequest> => {
      const res = await http.get<WebhookRequest>(`/api/requests/${requestId}`);
      return res.data;
    },
    body: async (requestId: string): Promise<string> => {
      const res = await http.get<string>(`/api/requests/${requestId}/body`, {
        responseType: 'text',
      });
      return res.data;
    },
    replay: async (requestId: string, targetUrl?: string): Promise<void> => {
      await http.post(`/api/requests/${requestId}/replay`, {
        target_url: targetUrl ?? undefined,
      });
    },
    replayResult: async (requestId: string): Promise<ReplayResult | null> => {
      try {
        const res = await http.get<ReplayResult>(`/api/requests/${requestId}/replay/result`);
        return res.data;
      } catch {
        return null;
      }
    },
  },
};
