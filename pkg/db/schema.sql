CREATE TABLE users
(
    created_at     timestamp DEFAULT now() NOT NULL,
    email          text                    NOT NULL,
    email_verified boolean   DEFAULT false NOT NULL,
    id             text PRIMARY KEY        NOT NULL,
    image          text,
    name           text                    NOT NULL,
    updated_at     timestamp DEFAULT now() NOT NULL,
    ban_expires    timestamp(6) with time zone,
    ban_reason     text,
    banned         boolean,
    role           text,
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE TABLE tenants
(
    id                      uuid                     default gen_random_uuid()                   NOT NULL
        PRIMARY KEY,
    name                    varchar(255)                                                         NOT NULL,
    slug                    varchar(100)                                                         NOT NULL
        UNIQUE,
    timezone                varchar(50)              default 'America/Bogota'::character varying NOT NULL,
    currency                varchar(3)               default 'COP'::character varying            NOT NULL,
    plan                    varchar(20)              default 'free'::character varying           NOT NULL,
    plan_expires_at         timestamp with time zone,
    appointments_this_month integer                  default 0                                   NOT NULL,
    month_reset_at          timestamp with time zone                                             NOT NULL,
    is_active               boolean                  default true                                NOT NULL,
    settings                jsonb                    default '{}'::jsonb,
    created_at              timestamp with time zone default now()                               NOT NULL,
    updated_at              timestamp with time zone default now()                               NOT NULL
);

CREATE TYPE whatsapp_activation_status AS ENUM (
    'pending',
    'in_progress',
    'active',
    'failed'
    );

CREATE TABLE tenant_whatsapp_configs
(
    id                       uuid                       default gen_random_uuid()                     NOT NULL
        PRIMARY KEY,
    tenant_id                uuid                                                                     NOT NULL
        UNIQUE
        REFERENCES tenants
            ON DELETE CASCADE,
    waba_id                  varchar(100),
    phone_number_id          varchar(100)
        UNIQUE,
    display_phone_number     varchar(20),
    access_token             text,
    token_expires_at         timestamp with time zone,
    is_active                boolean                    default false                                 NOT NULL,
    verified_at              timestamp with time zone,
    created_at               timestamp with time zone   default now()                                 NOT NULL,
    updated_at               timestamp with time zone   default now()                                 NOT NULL,
    activation_status        whatsapp_activation_status default 'pending'::whatsapp_activation_status NOT NULL,
    activation_requested_at  timestamp with time zone,
    activation_notes         text,
    activation_contact_email text
);

CREATE TYPE appointment_status AS ENUM (
    'pending',
    'confirmed',
    'in_progress',
    'completed',
    'cancelled',
    'no_show',
    'check_in'
    );

CREATE TABLE appointments
(
    id                   uuid                     default gen_random_uuid()             NOT NULL
        PRIMARY KEY,
    tenant_id            uuid                                                           NOT NULL
        REFERENCES tenants,
    resource_id          uuid                                                           NOT NULL
        REFERENCES resources,
    service_id           uuid                                                           NOT NULL
        REFERENCES services,
    customer_id          uuid                                                           NOT NULL
        CONSTRAINT appointments_client_id_fkey
            REFERENCES customers,
    starts_at            timestamp with time zone                                       NOT NULL,
    ends_at              timestamp with time zone                                       NOT NULL,
    status               appointment_status       default 'pending'::appointment_status NOT NULL,
    cancelled_by         text,
    cancel_reason        varchar(500),
    price_at_booking     numeric(10, 2)                                                 NOT NULL,
    reminder_24h_sent_at timestamp with time zone,
    reminder_1h_sent_at  timestamp with time zone,
    notes                varchar(500),
    created_at           timestamp with time zone default now()                         NOT NULL,
    updated_at           timestamp with time zone default now()                         NOT NULL,
    cancelled_at         timestamp with time zone,
    completed_at         timestamp with time zone,
    CONSTRAINT no_overlap
        EXCLUDE USING gist (resource_id with =, tstzrange(starts_at, ends_at) with &&),
    CONSTRAINT no_customer_overlap
        EXCLUDE USING gist (tenant_id with =, customer_id with =, tstzrange(starts_at, ends_at) with &&)
            WHERE (status <> ALL (ARRAY ['cancelled'::appointment_status, 'no_show'::appointment_status]))
);

CREATE TABLE appointment_status_history
(
    id              uuid                     default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    appointment_id  uuid                                               NOT NULL
        REFERENCES appointments,
    from_status     appointment_status                                 NOT NULL,
    to_status       appointment_status                                 NOT NULL,
    changed_by      text
        REFERENCES users,
    changed_by_role varchar(20),
    reason          varchar(500),
    created_at      timestamp with time zone default now()             NOT NULL
);

CREATE TABLE appointment_penalty_events
(
    id             uuid                     default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    appointment_id uuid                                               NOT NULL
        REFERENCES appointments
            ON DELETE CASCADE,
    tenant_id      uuid                                               NOT NULL
        REFERENCES tenants
            ON DELETE CASCADE,
    customer_id    uuid                                               NOT NULL
        REFERENCES customers
            ON DELETE CASCADE,
    event_type     varchar(20)                                        NOT NULL,
    occurred_at    timestamp with time zone                           NOT NULL,
    created_at     timestamp with time zone default now()             NOT NULL,
    CONSTRAINT appointment_penalty_events_unique UNIQUE (appointment_id, event_type)
);

CREATE TABLE appointment_reminder_events
(
    id              uuid                     default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    appointment_id  uuid                                               NOT NULL
        REFERENCES appointments
            ON DELETE CASCADE,
    tenant_id       uuid                                               NOT NULL
        REFERENCES tenants
            ON DELETE CASCADE,
    customer_id     uuid                                               NOT NULL
        REFERENCES customers
            ON DELETE CASCADE,
    reminder_type   varchar(10)                                        NOT NULL,
    attempts        integer                  default 0                 NOT NULL,
    sent_at         timestamp with time zone,
    last_attempt_at timestamp with time zone,
    last_error      text,
    created_at      timestamp with time zone default now()             NOT NULL,
    CONSTRAINT appointment_reminder_events_unique UNIQUE (appointment_id, reminder_type)
);

CREATE TABLE conversation_sessions
(
    id                 uuid                     DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
    tenant_id          uuid                                               NOT NULL REFERENCES tenants,
    whatsapp_config_id uuid                                               NOT NULL REFERENCES tenant_whatsapp_configs,
    customer_id        uuid                                               NOT NULL
        CONSTRAINT conversation_sessions_client_id_fkey REFERENCES customers,
    step               varchar(50)                                        NOT NULL,
    data               jsonb                    DEFAULT '{}'::jsonb       NOT NULL,
    expires_at         timestamp with time zone                           NOT NULL,
    created_at         timestamp with time zone DEFAULT now()             NOT NULL,
    updated_at         timestamp with time zone DEFAULT now()             NOT NULL,
    CONSTRAINT conversation_sessions_tenant_id_client_id_key UNIQUE (tenant_id, customer_id)
);

CREATE TABLE customers
(
    id                uuid                     default gen_random_uuid() NOT NULL
        CONSTRAINT clients_pkey PRIMARY KEY,
    tenant_id         uuid                                               NOT NULL
        CONSTRAINT clients_tenant_id_fkey REFERENCES tenants ON DELETE CASCADE,
    phone_number      varchar(20)                                        NOT NULL,
    name              varchar(255),
    is_blocked        boolean                  default false             NOT NULL,
    created_at        timestamp with time zone default now()             NOT NULL,
    no_show_count     integer                  default 0                 NOT NULL,
    late_cancel_count integer                  default 0                 NOT NULL,
    CONSTRAINT clients_tenant_id_phone_number_key
        UNIQUE (tenant_id, phone_number)
);

CREATE TABLE onboarding_progress
(
    id           uuid                     default gen_random_uuid() NOT NULL PRIMARY KEY,
    tenant_id    uuid                                               NOT NULL UNIQUE REFERENCES tenants ON DELETE CASCADE,
    current_step integer                  default 1                 NOT NULL,
    completed_at timestamp with time zone,
    created_at   timestamp with time zone default now()             NOT NULL,
    updated_at   timestamp with time zone default now()             NOT NULL
);

CREATE TABLE resources
(
    id         uuid                     default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    tenant_id  uuid                                               NOT NULL
        REFERENCES tenants
            ON DELETE CASCADE,
    name       varchar(255)                                       NOT NULL,
    type       varchar(50)                                        NOT NULL,
    avatar_url varchar(500),
    is_active  boolean                  default true              NOT NULL,
    sort_order integer                  default 0                 NOT NULL,
    created_at timestamp with time zone default now()             NOT NULL
);

CREATE TABLE working_hours
(
    id          uuid    default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    resource_id uuid                              NOT NULL
        REFERENCES resources
            ON DELETE CASCADE,
    day_of_week smallint                          NOT NULL
        CONSTRAINT working_hours_day_of_week_check
            check ((day_of_week >= 0) AND (day_of_week <= 6)),
    start_time  time                              NOT NULL,
    end_time    time                              NOT NULL,
    is_active   boolean default true              NOT NULL,
    CONSTRAINT uq_working_hours_resource_day
        UNIQUE (resource_id, day_of_week)
);

CREATE TABLE schedule_overrides
(
    id          uuid                     default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    resource_id uuid                                               NOT NULL
        REFERENCES resources
            ON DELETE CASCADE,
    date        date                                               NOT NULL,
    is_day_off  boolean                  default false             NOT NULL,
    start_time  time,
    end_time    time,
    reason      varchar(255),
    created_at  timestamp with time zone default now()             NOT NULL,
    CONSTRAINT uq_schedule_overrides_resource_date
        UNIQUE (resource_id, date)
);

CREATE TABLE services
(
    id               uuid                     default gen_random_uuid() NOT NULL
        PRIMARY KEY,
    tenant_id        uuid                                               NOT NULL
        REFERENCES tenants
            ON DELETE CASCADE,
    name             varchar(255)                                       NOT NULL,
    description      varchar(500),
    duration_minutes integer                                            NOT NULL,
    buffer_minutes   integer                  default 0                 NOT NULL,
    price            numeric(10, 2)                                     NOT NULL,
    is_active        boolean                  default true              NOT NULL,
    sort_order       integer                  default 0                 NOT NULL,
    created_at       timestamp with time zone default now()             NOT NULL
);

CREATE TABLE resource_services
(
    resource_id uuid NOT NULL
        REFERENCES resources
            ON DELETE CASCADE,
    service_id  uuid NOT NULL
        REFERENCES services
            ON DELETE CASCADE,
    PRIMARY KEY (resource_id, service_id)
);

CREATE TABLE tenant_users
(
    user_id   text NOT NULL
        CONSTRAINT tenant_users_user_id_users_id_fk
            REFERENCES users
            ON DELETE CASCADE,
    tenant_id uuid NOT NULL
        CONSTRAINT tenant_users_tenant_id_tenants_id_fk
            REFERENCES tenants
            ON DELETE CASCADE,
    role      text NOT NULL,
    PRIMARY KEY (user_id, tenant_id)
);

CREATE INDEX idx_appointments_reminder
    ON appointments (starts_at, reminder_24h_sent_at, reminder_1h_sent_at)
    WHERE (status = 'confirmed'::appointment_status);

CREATE INDEX idx_appointments_status_date
    ON appointments (tenant_id, status, starts_at);
CREATE INDEX idx_appointments_cancelled_recent
    ON appointments (cancelled_at DESC)
    WHERE status = 'cancelled'::appointment_status
      AND cancelled_at IS NOT NULL;
CREATE INDEX idx_appointments_unattended
    ON appointments (starts_at)
    WHERE status = 'confirmed'::appointment_status;
CREATE INDEX idx_status_history_appointment ON appointment_status_history (appointment_id);
CREATE INDEX idx_appointment_penalty_events_customer
    ON appointment_penalty_events (tenant_id, customer_id, occurred_at DESC);
CREATE INDEX idx_appointment_reminder_events_pending
    ON appointment_reminder_events (sent_at, attempts, created_at)
    WHERE sent_at IS NULL;
CREATE INDEX idx_sessions_active_lookup
    ON conversation_sessions (tenant_id, customer_id, expires_at);
