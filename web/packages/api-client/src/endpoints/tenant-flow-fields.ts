import { defineResource } from "../core/define-resource"
import type { EndpointDefinition } from "../core/types"
import type { TenantFlowField } from "../types/tenant-flow-fields"

const definitions = {
  list: {
    method: "GET",
    path: "/tenants/flow-fields",
  } as EndpointDefinition<TenantFlowField[]>,
}

export const tenantFlowFieldEndpoints = defineResource(definitions)
