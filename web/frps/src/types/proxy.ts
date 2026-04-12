export interface ProxyStatsInfo {
  name: string
  conf: any
  user: string
  clientID: string
  todayTrafficIn: number
  todayTrafficOut: number
  curConns: number
  lastStartTime: string
  lastCloseTime: string
  status: string
}

export interface GetProxyResponse {
  proxies: ProxyStatsInfo[]
}

export interface TrafficResponse {
  name: string
  trafficIn: number[]
  trafficOut: number[]
}

export interface AccessLogRecord {
  id: number
  proxyName: string
  proxyType: string
  proxyUser: string
  remoteIP: string
  remotePort: number
  connectedAt: number   // unix ms
  duration: number      // ms
  trafficIn: number     // bytes
  trafficOut: number    // bytes
  userAgent?: string
  host?: string
  url?: string
  statusCode?: number
  blocked?: boolean
}

export interface AccessLogResult {
  total: number
  records: AccessLogRecord[]
}

export interface AccessLogQueryParams {
  proxyName?: string
  remoteIP?: string
  startTime?: number  // unix ms
  endTime?: number    // unix ms
  page?: number
  pageSize?: number
}
