export interface ClientInfoData {
  key: string
  user: string
  clientID: string
  runID: string
  version?: string
  hostname: string
  clientIP?: string
  os?: string
  arch?: string
  metas?: Record<string, string>
  poolCount?: number
  firstConnectedAt: number
  lastConnectedAt: number
  disconnectedAt?: number
  online: boolean
}
