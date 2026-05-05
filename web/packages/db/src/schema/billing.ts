import { sql } from "drizzle-orm"
import {
  boolean,
  index,
  integer,
  jsonb,
  pgTable,
  text,
  timestamp,
  unique,
  uniqueIndex,
  uuid,
  varchar,
} from "drizzle-orm/pg-core"

import { tenants } from "./tenants"

export const plans = pgTable(
  "plans",
  {
    id: uuid().defaultRandom().primaryKey(),
    externalId: varchar("external_id", { length: 100 }).notNull(),
    externalPriceId: varchar("external_price_id", { length: 100 }),
    name: varchar({ length: 100 }).notNull(),
    description: text(),
    price: integer().default(0).notNull(),
    currency: varchar({ length: 3 }).default("COP").notNull(),
    interval: varchar({ length: 20 }),
    features: jsonb().default({}).notNull(),
    isActive: boolean("is_active").default(true).notNull(),
    createdAt: timestamp("created_at", { withTimezone: true })
      .default(sql`now()`)
      .notNull(),
    updatedAt: timestamp("updated_at", { withTimezone: true })
      .default(sql`now()`)
      .notNull(),
    environment: varchar({ length: 20 }).default("production").notNull(),
  },
  (table) => [
    uniqueIndex("uq_plans_external_id_environment").using(
      "btree",
      table.externalId.asc().nullsLast(),
      table.environment.asc().nullsLast()
    ),
    unique("plans_external_id_key").on(table.externalId),
  ]
)

export const subscriptions = pgTable(
  "subscriptions",
  {
    id: uuid().defaultRandom().primaryKey(),
    tenantId: uuid("tenant_id")
      .notNull()
      .references(() => tenants.id),
    planId: uuid("plan_id")
      .notNull()
      .references(() => plans.id),
    externalId: varchar("external_id", { length: 100 }).notNull(),
    externalCustomerId: varchar("external_customer_id", {
      length: 100,
    }).notNull(),
    status: varchar({ length: 20 }).default("pending").notNull(),
    currentPeriodStart: timestamp("current_period_start", {
      withTimezone: true,
    }),
    currentPeriodEnd: timestamp("current_period_end", { withTimezone: true }),
    cancelAtPeriodEnd: boolean("cancel_at_period_end").default(false).notNull(),
    canceledAt: timestamp("canceled_at", { withTimezone: true }),
    createdAt: timestamp("created_at", { withTimezone: true })
      .default(sql`now()`)
      .notNull(),
    updatedAt: timestamp("updated_at", { withTimezone: true })
      .default(sql`now()`)
      .notNull(),
    environment: varchar({ length: 20 }).default("production").notNull(),
  },
  (table) => [
    index("idx_subscriptions_external_id").using(
      "btree",
      table.externalId.asc().nullsLast(),
      table.environment.asc().nullsLast()
    ),
    uniqueIndex("uq_tenant_active_subscription")
      .using(
        "btree",
        table.tenantId.asc().nullsLast(),
        table.environment.asc().nullsLast()
      )
      .where(sql`((status)::text = 'active'::text)`),
    unique("subscriptions_external_id_key").on(table.externalId),
  ]
)

export const subscriptionOrders = pgTable(
  "subscription_orders",
  {
    id: uuid().defaultRandom().primaryKey(),
    subscriptionId: uuid("subscription_id")
      .notNull()
      .references(() => subscriptions.id),
    externalId: varchar("external_id", { length: 100 }).notNull(),
    amount: integer().notNull(),
    currency: varchar({ length: 3 }).default("COP").notNull(),
    status: varchar({ length: 20 }).notNull(),
    createdAt: timestamp("created_at", { withTimezone: true })
      .default(sql`now()`)
      .notNull(),
    environment: varchar({ length: 20 }).default("production").notNull(),
  },
  (table) => [
    uniqueIndex("uq_subscription_orders_external_id_environment").using(
      "btree",
      table.externalId.asc().nullsLast(),
      table.environment.asc().nullsLast()
    ),
    unique("subscription_orders_external_id_key").on(table.externalId),
  ]
)
