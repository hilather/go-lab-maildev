export type Problem = {
  type: string;
  title: string;
  status: number;
  detail: string;
  code: string;
};

export type SessionCreated = {
  csrf: string;
  expiresAt: string;
};

export type SessionView = {
  id: string;
  role: string;
  scopes: string[];
  csrf?: string;
  expiresAt?: string;
};

export type Address = {
  name: string;
  address: string;
};

export type Header = {
  name: string;
  value: string;
};

export type Attachment = {
  id: string;
  filename: string;
  contentType: string;
  contentId?: string;
  disposition?: string;
  size: number;
  checksum: string;
};

export type Envelope = {
  from: string;
  to: string[];
  helo: string;
  remoteAddress: string;
  tls: boolean;
  authUser?: string;
};

export type Message = {
  id: string;
  receivedAt: string;
  subject: string;
  from: Address[];
  to: Address[];
  cc: Address[];
  bcc: Address[];
  replyTo?: Address[];
  messageId: string;
  inReplyTo?: string;
  date?: string;
  read: boolean;
  size: number;
  priority?: string;
  parseWarning?: string;
  hasHTML: boolean;
  envelope: Envelope;
  headers?: Header[];
  attachments: Attachment[];
  text?: string;
  html?: string;
};

export type MessageList = {
  revision: string;
  storeGeneration: number;
  items: Message[];
  nextCursor: string | null;
};

export type StoreStats = {
  messageCount: number;
  storeBytes: number;
  unreadCount: number;
  storeGeneration: number;
  epoch: number;
};

export type Listener = {
  name: string;
  address: string;
};

export type Status = {
  ready: boolean;
  revisions: Record<string, unknown>;
  listeners: Listener[];
  store: StoreStats;
};

export type AuditEvent = {
  id: string;
  time: string;
  actorId?: string;
  actorClass?: string;
  transport?: string;
  capability?: string;
  reason?: string;
  messageId?: string;
  previous?: string;
  revision?: string;
  storeGeneration?: number;
  result?: string;
  errorCode?: string;
};

export type AuditList = {
  events: AuditEvent[];
};
