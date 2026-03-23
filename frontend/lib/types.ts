export interface Endpoint {
  id: string;
  name: string;
  target_url: string;
  created_at: string;
  expires_at: string;
  response_status: number;
  response_content_type: string;
  response_headers: Record<string, string>;
  response_body: string;
  notify_email: string;
}

export interface WebhookRequest {
  id: string;
  endpoint_id: string;
  method: string;
  headers: Record<string, string[]>;
  query_params: Record<string, string[]>;
  content_type: string;
  body_size: number;
  body_preview: string;
  s3_key: string;
  created_at: string;
}

export interface ReplayResult {
  id: string;
  request_id: string;
  status_code: number;
  response_body: string;
  duration_ms: number;
  error?: string;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  plan: string;
  created_at: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface CreateEndpointPayload {
  name: string;
  target_url?: string;
}

export interface EndpointResponseConfig {
  status: number;
  content_type: string;
  headers: Record<string, string>;
  body: string;
}
