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
  create: (input: CreateWebhookInput) =>
    api.post<CreateWebhookResponse>("/admin/webhooks", input),
  update: (id: string, input: UpdateWebhookInput) =>
    api.patch<Webhook>(`/admin/webhooks/${id}`, input),
  delete: (id: string) => api.delete<void>(`/admin/webhooks/${id}`),
  listDeliveries: (params?: {
    subscription_id?: string;
    status?: string;
    event_type?: string;
    limit?: number;
  }) => {
    const query = new URLSearchParams();
    if (params?.subscription_id)
      query.set("subscription_id", params.subscription_id);
    if (params?.status) query.set("status", params.status);
    if (params?.event_type) query.set("event_type", params.event_type);
    if (params?.limit) query.set("limit", params.limit.toString());
    const qs = query.toString();
    return api.get<WebhookDelivery[]>(
      `/admin/webhooks/deliveries${qs ? `?${qs}` : ""}`,
    );
  },
};
