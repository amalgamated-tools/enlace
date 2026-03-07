import { api } from "./client";

export interface Webhook {
  id: string;
  name: string;
  url: string;
  events: string[];
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateWebhookResponse extends Webhook {
  secret: string;
}

export interface WebhookDelivery {
  id: string;
  subscription_id: string;
  event_type: string;
  event_id: string;
  idempotency_key: string;
  attempt: number;
  status: string;
  status_code?: number;
  next_attempt_at?: string;
  delivered_at?: string;
  error?: string;
  request_body?: string;
  duration_ms: number;
  created_at: string;
}

export interface CreateWebhookInput {
  name: string;
  url: string;
  events: string[];
}

export interface UpdateWebhookInput {
  name?: string;
  url?: string;
  events?: string[];
  enabled?: boolean;
}

export const webhooksApi = {
  list: () => api.get<Webhook[]>("/admin/webhooks"),
  listEvents: () => api.get<string[]>("/admin/webhooks/events"),
  create: (input: CreateWebhookInput) =>
    api.post<CreateWebhookResponse>("/admin/webhooks", input),
  update: (id: string, input: UpdateWebhookInput) =>
    api.patch<Webhook>(`/admin/webhooks/${id}`, input),
  delete: (id: string) => api.delete<void>(`/admin/webhooks/${id}`),
  listDeliveries: (params?: Record<string, string | number | undefined>) => {
    const query = new URLSearchParams();
    if (params) {
      for (const [key, value] of Object.entries(params)) {
        if (value !== undefined) query.set(key, String(value));
      }
    }
    const qs = query.toString();
    return api.get<WebhookDelivery[]>(
      `/admin/webhooks/deliveries${qs ? `?${qs}` : ""}`,
    );
  },
};
