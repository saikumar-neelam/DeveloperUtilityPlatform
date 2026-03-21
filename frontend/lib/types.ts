export interface Endpoint {
  id: string;
  name: string;
  target_url: string;
  created_at: string;
  expires_at: string;
}

export interface WebhookRequest {
  id: string;
  endpoint_id: string;
  method: string;
  headers: Record<string, string[]>;
  query_params: Record<string, string[]>;
  content_type: string;
  body_size: number;
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

export interface CreateEndpointPayload {
  name: string;
  target_url?: string;
}
