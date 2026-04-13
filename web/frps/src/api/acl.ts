import { http } from './http'

export interface GlobalACL {
  allowIPs: string[]
  denyIPs: string[]
  allowUserAgents: string[]
  denyUserAgents: string[]
}

export interface GlobalACLResp extends GlobalACL {
  /** Whether the change was successfully persisted to the config file on disk.
   *  false means effective in memory only — will be lost after restart. */
  persisted: boolean
}

export const getGlobalACL = () => http.get<GlobalACLResp>('../api/config/globalACL')

export const updateGlobalACL = (body: GlobalACL) =>
  http.put<GlobalACLResp>('../api/config/globalACL', body)
