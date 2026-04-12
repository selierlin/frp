import { http } from './http'
import type {
  GetProxyResponse,
  ProxyStatsInfo,
  TrafficResponse,
  AccessLogQueryParams,
  AccessLogResult,
} from '../types/proxy'

export const getProxiesByType = (type: string) => {
  return http.get<GetProxyResponse>(`../api/proxy/${type}`)
}

export const getProxy = (type: string, name: string) => {
  return http.get<ProxyStatsInfo>(`../api/proxy/${type}/${name}`)
}

export const getProxyByName = (name: string) => {
  return http.get<ProxyStatsInfo>(`../api/proxies/${name}`)
}

export const getProxyTraffic = (name: string) => {
  return http.get<TrafficResponse>(`../api/traffic/${name}`)
}

export const clearOfflineProxies = () => {
  return http.delete('../api/proxies?status=offline')
}

export const getAccessLog = (params: AccessLogQueryParams) => {
  const query = new URLSearchParams()
  if (params.proxyName) query.set('proxyName', params.proxyName)
  if (params.remoteIP) query.set('remoteIP', params.remoteIP)
  if (params.event) query.set('event', params.event)
  if (params.startTime) query.set('startTime', String(params.startTime))
  if (params.endTime) query.set('endTime', String(params.endTime))
  query.set('page', String(params.page ?? 1))
  query.set('pageSize', String(params.pageSize ?? 20))
  return http.get<AccessLogResult>(`../api/accesslog?${query.toString()}`)
}
