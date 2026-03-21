import axios from 'axios';
import type { Endpoint, WebhookRequest } from './types';

// Requests go to localhost:3000 which Next.js rewrites to the backend.
const http = axios.create({ baseURL: '/' });

export const api = {
  endpoints: {
    list: async (): Promise<Endpoint[]> => {
      const res = await http.get<Endpoint[]>('/api/endpoints');
      return res.data;
    },
    get: async (id: string): Promise<Endpoint> => {
      const res = await http.get<Endpoint>(`/api/endpoints/${id}`);
      return res.data;
    },
    create: async (targetUrl?: string): Promise<Endpoint> => {
      const res = await http.post<Endpoint>('/api/endpoints', { target_url: targetUrl ?? '' });
      return res.data;
    },
    delete: async (id: string): Promise<void> => {
      await http.delete(`/api/endpoints/${id}`);
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
  },
};
