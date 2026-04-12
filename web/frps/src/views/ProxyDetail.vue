<template>
  <div class="proxy-detail-page">
    <!-- Breadcrumb -->
    <nav class="breadcrumb">
      <a class="breadcrumb-link" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </a>
      <template v-if="fromClient">
        <router-link to="/clients" class="breadcrumb-item">Clients</router-link>
        <span class="breadcrumb-separator">/</span>
        <router-link :to="`/clients/${fromClient}`" class="breadcrumb-item">{{
          fromClient
        }}</router-link>
        <span class="breadcrumb-separator">/</span>
      </template>
      <template v-else>
        <router-link to="/proxies" class="breadcrumb-item">Proxies</router-link>
        <span class="breadcrumb-separator">/</span>
        <router-link
          v-if="proxy?.clientID"
          :to="clientLink"
          class="breadcrumb-item"
        >
          {{ proxy.user ? `${proxy.user}.${proxy.clientID}` : proxy.clientID }}
        </router-link>
        <span v-if="proxy?.clientID" class="breadcrumb-separator">/</span>
      </template>
      <span class="breadcrumb-current">{{ proxyName }}</span>
    </nav>

    <div v-loading="loading" class="detail-content">
      <template v-if="proxy">
        <!-- Header Section -->
        <div class="header-section">
          <div class="header-main">
            <div
              class="proxy-icon"
              :style="{ background: proxyIconConfig.gradient }"
            >
              <el-icon><component :is="proxyIconConfig.icon" /></el-icon>
            </div>
            <div class="header-info">
              <div class="header-title-row">
                <h1 class="proxy-name">{{ proxy.name }}</h1>
                <span class="type-tag">{{ proxy.type.toUpperCase() }}</span>
                <span class="status-badge" :class="proxy.status">
                  {{ proxy.status }}
                </span>
              </div>
              <div class="header-meta">
                <router-link
                  v-if="proxy.clientID"
                  :to="clientLink"
                  class="meta-link"
                >
                  <el-icon><Monitor /></el-icon>
                  <span>{{
                    proxy.user
                      ? `${proxy.user}.${proxy.clientID}`
                      : proxy.clientID
                  }}</span>
                </router-link>
                <span v-if="proxy.lastStartTime" class="meta-text">
                  <span class="meta-sep">·</span>
                  Last Started {{ proxy.lastStartTime }}
                </span>
                <span v-if="proxy.lastCloseTime" class="meta-text">
                  <span class="meta-sep">·</span>
                  Last Closed {{ proxy.lastCloseTime }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- Stats Bar -->
        <div class="stats-bar">
          <div v-if="proxy.port" class="stats-item">
            <span class="stats-label">Port</span>
            <span class="stats-value">{{ proxy.port }}</span>
          </div>
          <div class="stats-item">
            <span class="stats-label">Connections</span>
            <span class="stats-value conn-link" @click="scrollToAccessLog">{{ proxy.conns }}</span>
          </div>
          <div class="stats-item">
            <span class="stats-label">Traffic</span>
            <span class="stats-value">↓ {{ formatTrafficValue(proxy.trafficIn) }} <small>{{ formatTrafficUnit(proxy.trafficIn) }}</small> / ↑ {{ formatTrafficValue(proxy.trafficOut) }} <small>{{ formatTrafficUnit(proxy.trafficOut) }}</small></span>
          </div>
        </div>

        <!-- Configuration Section -->
        <div class="config-section">
          <div class="config-section-header">
            <el-icon><Setting /></el-icon>
            <h2>Configuration</h2>
          </div>

          <!-- Config Cards Grid -->
          <div class="config-grid">
            <div class="config-item-card">
              <div class="config-item-icon encryption">
                <el-icon><Lock /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Encryption</span>
                <span class="config-item-value">{{
                  proxy.encryption ? 'Enabled' : 'Disabled'
                }}</span>
              </div>
            </div>

            <div class="config-item-card">
              <div class="config-item-icon compression">
                <el-icon><Lightning /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Compression</span>
                <span class="config-item-value">{{
                  proxy.compression ? 'Enabled' : 'Disabled'
                }}</span>
              </div>
            </div>

            <div v-if="proxy.localPort > 0" class="config-item-card">
              <div class="config-item-icon backend">
                <el-icon><Cpu /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Backend</span>
                <span class="config-item-value">{{ proxy.localIP }}:{{ proxy.localPort }}</span>
              </div>
            </div>

            <div v-if="proxy.customDomains" class="config-item-card">
              <div class="config-item-icon domains">
                <el-icon><Link /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Custom Domains</span>
                <span class="config-item-value">{{ proxy.customDomains }}</span>
              </div>
            </div>

            <div v-if="proxy.subdomain" class="config-item-card">
              <div class="config-item-icon subdomain">
                <el-icon><Link /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Subdomain</span>
                <span class="config-item-value">{{ proxy.subdomain }}</span>
              </div>
            </div>

            <div v-if="proxy.locations" class="config-item-card">
              <div class="config-item-icon locations">
                <el-icon><Location /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Locations</span>
                <span class="config-item-value">{{ proxy.locations }}</span>
              </div>
            </div>

            <div v-if="proxy.hostHeaderRewrite" class="config-item-card">
              <div class="config-item-icon host">
                <el-icon><Tickets /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Host Rewrite</span>
                <span class="config-item-value">{{
                  proxy.hostHeaderRewrite
                }}</span>
              </div>
            </div>

            <div v-if="proxy.multiplexer" class="config-item-card">
              <div class="config-item-icon multiplexer">
                <el-icon><Cpu /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Multiplexer</span>
                <span class="config-item-value">{{ proxy.multiplexer }}</span>
              </div>
            </div>

            <div v-if="proxy.routeByHTTPUser" class="config-item-card">
              <div class="config-item-icon route">
                <el-icon><Connection /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Route By HTTP User</span>
                <span class="config-item-value">{{
                  proxy.routeByHTTPUser
                }}</span>
              </div>
            </div>

            <div v-if="proxy.allowIPs && proxy.allowIPs.length > 0" class="config-item-card">
              <div class="config-item-icon allow-ip">
                <el-icon><CircleCheck /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Allow IPs</span>
                <div class="config-tag-list">
                  <span v-for="ip in proxy.allowIPs" :key="ip" class="config-tag config-tag--allow">{{ ip }}</span>
                </div>
              </div>
            </div>

            <div v-if="proxy.denyIPs && proxy.denyIPs.length > 0" class="config-item-card">
              <div class="config-item-icon deny-ip">
                <el-icon><CircleClose /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Deny IPs</span>
                <div class="config-tag-list">
                  <span v-for="ip in proxy.denyIPs" :key="ip" class="config-tag config-tag--deny">{{ ip }}</span>
                </div>
              </div>
            </div>

            <div v-if="proxy.allowUserAgents && proxy.allowUserAgents.length > 0" class="config-item-card">
              <div class="config-item-icon useragent">
                <el-icon><Monitor /></el-icon>
              </div>
              <div class="config-item-content">
                <span class="config-item-label">Allow User-Agents</span>
                <div class="config-tag-list">
                  <span v-for="ua in proxy.allowUserAgents" :key="ua" class="config-tag config-tag--ua">{{ ua }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Annotations -->
          <template v-if="proxy.annotations && proxy.annotations.size > 0">
            <div class="annotations-section">
              <div
                v-for="[key, value] in proxy.annotations"
                :key="key"
                class="annotation-tag"
              >
                {{ key }}: {{ value }}
              </div>
            </div>
          </template>
        </div>

        <!-- Traffic Card -->
        <div class="traffic-card">
          <div class="traffic-header">
            <h2>Traffic Statistics</h2>
          </div>
          <div class="traffic-body">
            <Traffic :proxy-name="proxyName" />
          </div>
        </div>

        <!-- Access Log Section -->
        <div class="access-log-card" ref="accessLogRef">
          <div class="access-log-header">
            <div class="access-log-title">
              <el-icon><List /></el-icon>
              <h2>Access Log</h2>
            </div>
            <div class="access-log-filters">
              <el-input
                v-model="logFilter.remoteIP"
                placeholder="Filter by IP"
                clearable
                size="small"
                style="width: 160px"
                @change="fetchAccessLog(1)"
                @clear="fetchAccessLog(1)"
              />
              <el-select
                v-model="logFilter.event"
                size="small"
                style="width: 130px"
                @change="fetchAccessLog(1)"
              >
                <el-option label="All Events" value="" />
                <el-option label="Active" value="connected" />
                <el-option label="Closed" value="disconnected" />
                <el-option label="Blocked" value="blocked" />
              </el-select>
              <el-date-picker
                v-model="logFilter.timeRange"
                type="datetimerange"
                size="small"
                range-separator="~"
                start-placeholder="Start"
                end-placeholder="End"
                :shortcuts="dateShortcuts"
                @change="fetchAccessLog(1)"
              />
              <el-button size="small" :icon="Refresh" circle @click="fetchAccessLog(logPage)" />
            </div>
          </div>

          <div class="access-log-body" v-loading="logLoading">
            <el-table
              :data="logRecords"
              size="small"
              stripe
              empty-text="No records"
              style="width: 100%"
            >
              <el-table-column label="Event" width="90" align="center">
                <template #default="{ row }">
                  <el-tag v-if="row.event === 'connected'" type="primary" size="small">Active</el-tag>
                  <el-tag v-else-if="row.event === 'disconnected'" type="info" size="small">Closed</el-tag>
                  <el-tag v-else-if="row.event === 'blocked'" type="danger" size="small">Blocked</el-tag>
                  <span v-else>—</span>
                </template>
              </el-table-column>
              <el-table-column label="Time" width="170">
                <template #default="{ row }">
                  {{ formatTime(row.connectedAt) }}
                </template>
              </el-table-column>
              <el-table-column label="Remote IP" width="140">
                <template #default="{ row }">
                  <span class="log-ip" @click="filterByIP(row.remoteIP)">{{ row.remoteIP }}</span>
                  <span class="log-port">:{{ row.remotePort }}</span>
                </template>
              </el-table-column>
              <el-table-column label="Duration" width="100">
                <template #default="{ row }">
                  {{ formatDuration(row.duration) }}
                </template>
              </el-table-column>
              <el-table-column label="Traffic ↓/↑" width="160">
                <template #default="{ row }">
                  <span class="traffic-in">{{ formatBytes(row.trafficIn) }}</span>
                  <span class="traffic-sep"> / </span>
                  <span class="traffic-out">{{ formatBytes(row.trafficOut) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="Status" width="80">
                <template #default="{ row }">
                  <el-tag
                    v-if="row.blocked"
                    type="danger"
                    size="small"
                    effect="plain"
                  >Blocked</el-tag>
                  <el-tag
                    v-else-if="row.statusCode && row.statusCode >= 400"
                    type="warning"
                    size="small"
                    effect="plain"
                  >{{ row.statusCode }}</el-tag>
                  <el-tag
                    v-else-if="row.statusCode"
                    type="success"
                    size="small"
                    effect="plain"
                  >{{ row.statusCode }}</el-tag>
                  <span v-else class="log-ok">—</span>
                </template>
              </el-table-column>
              <el-table-column label="Host / URL" min-width="160" show-overflow-tooltip>
                <template #default="{ row }">
                  <span v-if="row.host" class="log-host">{{ row.host }}{{ row.url }}</span>
                  <span v-else class="log-empty">—</span>
                </template>
              </el-table-column>
              <el-table-column label="User-Agent" min-width="160" show-overflow-tooltip>
                <template #default="{ row }">
                  <span v-if="row.userAgent" class="log-ua">{{ row.userAgent }}</span>
                  <span v-else class="log-empty">—</span>
                </template>
              </el-table-column>
            </el-table>

            <div class="access-log-pagination">
              <el-pagination
                v-model:current-page="logPage"
                v-model:page-size="logPageSize"
                :total="logTotal"
                :page-sizes="[20, 50, 100]"
                layout="total, sizes, prev, pager, next"
                small
                @current-change="fetchAccessLog"
                @size-change="fetchAccessLog(1)"
              />
            </div>
          </div>
        </div>
      </template>

      <div v-else-if="!loading" class="not-found">
        <h2>Proxy not found</h2>
        <p>The proxy doesn't exist or has been removed.</p>
        <router-link to="/proxies">
          <el-button type="primary">Back to Proxies</el-button>
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  ArrowLeft,
  Monitor,
  Connection,
  Link,
  Lock,
  Promotion,
  Grid,
  Setting,
  Cpu,
  Lightning,
  Tickets,
  Location,
  CircleCheck,
  CircleClose,
  List,
  Refresh,
} from '@element-plus/icons-vue'
import { getProxyByName, getAccessLog } from '../api/proxy'
import { getServerInfo } from '../api/server'
import {
  BaseProxy,
  TCPProxy,
  UDPProxy,
  HTTPProxy,
  HTTPSProxy,
  TCPMuxProxy,
  STCPProxy,
  SUDPProxy,
} from '../utils/proxy'
import Traffic from '../components/Traffic.vue'
import type { AccessLogRecord } from '../types/proxy'

const route = useRoute()
const router = useRouter()
const proxyName = computed(() => route.params.name as string)
const fromClient = computed(() => {
  if (route.query.from === 'client' && route.query.client) {
    return route.query.client as string
  }
  return null
})
const proxy = ref<BaseProxy | null>(null)
const loading = ref(true)
const accessLogRef = ref<HTMLElement | null>(null)

const scrollToAccessLog = () => {
  accessLogRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// ── Access Log ──────────────────────────────────────────────
const logLoading = ref(false)
const logRecords = ref<AccessLogRecord[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logPageSize = ref(20)
const logFilter = ref<{ remoteIP: string; event: string; timeRange: [Date, Date] | null }>({
  remoteIP: '',
  event: '',
  timeRange: null,
})

const dateShortcuts = [
  {
    text: 'Last 1h',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setHours(start.getHours() - 1)
      return [start, end]
    },
  },
  {
    text: 'Last 24h',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setDate(start.getDate() - 1)
      return [start, end]
    },
  },
  {
    text: 'Last 7d',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setDate(start.getDate() - 7)
      return [start, end]
    },
  },
]

const fetchAccessLog = async (page = logPage.value) => {
  logLoading.value = true
  logPage.value = page
  try {
    const params: any = {
      proxyName: proxyName.value,
      page: logPage.value,
      pageSize: logPageSize.value,
    }
    if (logFilter.value.remoteIP) params.remoteIP = logFilter.value.remoteIP
    if (logFilter.value.event) params.event = logFilter.value.event
    if (logFilter.value.timeRange) {
      params.startTime = logFilter.value.timeRange[0].getTime()
      params.endTime = logFilter.value.timeRange[1].getTime()
    }
    const res = await getAccessLog(params)
    logRecords.value = res.records ?? []
    logTotal.value = res.total ?? 0
  } catch {
    // access log may not be enabled, silently skip
  } finally {
    logLoading.value = false
  }
}

const filterByIP = (ip: string) => {
  logFilter.value.remoteIP = ip
  fetchAccessLog(1)
}

const formatTime = (ms: number): string => {
  if (!ms) return '—'
  return new Date(ms).toLocaleString()
}

const formatDuration = (ms: number): string => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  return `${m}m${s}s`
}

const formatBytes = (bytes: number): string => {
  if (!bytes) return '0B'
  const units = ['B', 'K', 'M', 'G']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const v = bytes / Math.pow(1024, i)
  return (v < 10 ? v.toFixed(1) : Math.round(v)) + units[i]
}
// ────────────────────────────────────────────────────────────

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/proxies')
  }
}

let serverInfo: {
  vhostHTTPPort: number
  vhostHTTPSPort: number
  tcpmuxHTTPConnectPort: number
  subdomainHost: string
} | null = null

const clientLink = computed(() => {
  if (!proxy.value) return ''
  const key = proxy.value.user
    ? `${proxy.value.user}.${proxy.value.clientID}`
    : proxy.value.clientID
  return `/clients/${key}`
})

const proxyIconConfig = computed(() => {
  const type = proxy.value?.type?.toLowerCase() || ''
  const configs: Record<string, { icon: any; gradient: string }> = {
    tcp: {
      icon: Connection,
      gradient: 'linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%)',
    },
    udp: {
      icon: Promotion,
      gradient: 'linear-gradient(135deg, #8b5cf6 0%, #6d28d9 100%)',
    },
    http: {
      icon: Link,
      gradient: 'linear-gradient(135deg, #22c55e 0%, #16a34a 100%)',
    },
    https: {
      icon: Lock,
      gradient: 'linear-gradient(135deg, #14b8a6 0%, #0d9488 100%)',
    },
    stcp: {
      icon: Lock,
      gradient: 'linear-gradient(135deg, #f97316 0%, #ea580c 100%)',
    },
    sudp: {
      icon: Lock,
      gradient: 'linear-gradient(135deg, #f97316 0%, #ea580c 100%)',
    },
    tcpmux: {
      icon: Grid,
      gradient: 'linear-gradient(135deg, #06b6d4 0%, #0891b2 100%)',
    },
    xtcp: {
      icon: Connection,
      gradient: 'linear-gradient(135deg, #ec4899 0%, #db2777 100%)',
    },
  }
  return (
    configs[type] || {
      icon: Connection,
      gradient: 'linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%)',
    }
  )
})

const formatTrafficValue = (bytes: number): string => {
  if (bytes === 0) return '0'
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const value = bytes / Math.pow(k, i)
  return value < 10 ? value.toFixed(1) : Math.round(value).toString()
}

const formatTrafficUnit = (bytes: number): string => {
  if (bytes === 0) return 'B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return units[i]
}

const fetchServerInfo = async () => {
  if (serverInfo) return serverInfo
  const res = await getServerInfo()
  serverInfo = res
  return serverInfo
}

const fetchProxy = async () => {
  const name = proxyName.value
  if (!name) {
    loading.value = false
    return
  }

  try {
    const data = await getProxyByName(name)
    const info = await fetchServerInfo()
    const type = data.conf?.type || ''

    if (type === 'tcp') {
      proxy.value = new TCPProxy(data)
    } else if (type === 'udp') {
      proxy.value = new UDPProxy(data)
    } else if (type === 'http' && info?.vhostHTTPPort) {
      proxy.value = new HTTPProxy(data, info.vhostHTTPPort, info.subdomainHost)
    } else if (type === 'https' && info?.vhostHTTPSPort) {
      proxy.value = new HTTPSProxy(
        data,
        info.vhostHTTPSPort,
        info.subdomainHost,
      )
    } else if (type === 'tcpmux' && info?.tcpmuxHTTPConnectPort) {
      proxy.value = new TCPMuxProxy(
        data,
        info.tcpmuxHTTPConnectPort,
        info.subdomainHost,
      )
    } else if (type === 'stcp') {
      proxy.value = new STCPProxy(data)
    } else if (type === 'sudp') {
      proxy.value = new SUDPProxy(data)
    } else {
      proxy.value = new BaseProxy(data)
      proxy.value.type = type
    }
  } catch (error: any) {
    ElMessage.error('Failed to fetch proxy: ' + error.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchProxy()
  fetchAccessLog(1)
})
</script>

<style scoped>
.proxy-detail-page {
}

/* Breadcrumb */
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  margin-bottom: 24px;
}

.breadcrumb-link {
  display: flex;
  align-items: center;
  color: var(--text-secondary);
  cursor: pointer;
  transition: color 0.2s;
  margin-right: 4px;
}

.breadcrumb-link:hover {
  color: var(--text-primary);
}

.breadcrumb-item {
  color: var(--text-secondary);
  text-decoration: none;
  transition: color 0.2s;
}

.breadcrumb-item:hover {
  color: var(--el-color-primary);
}

.breadcrumb-separator {
  color: var(--el-border-color);
}

.breadcrumb-current {
  color: var(--text-primary);
  font-weight: 500;
}

/* Header Section */
.header-section {
  margin-bottom: 24px;
}

.header-main {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.proxy-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 26px;
  color: white;
}

.header-info {
  flex: 1;
  min-width: 0;
}

.header-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.proxy-name {
  font-size: 20px;
  font-weight: 500;
  color: var(--text-primary);
  margin: 0;
  line-height: 1.3;
  word-break: break-all;
}

.type-tag {
  font-size: 12px;
  font-weight: 500;
  padding: 4px 12px;
  border-radius: 20px;
  background: var(--el-fill-color-dark);
  color: var(--el-text-color-secondary);
  border: 1px solid var(--el-border-color-lighter);
}

.status-badge {
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  text-transform: capitalize;
}

.status-badge.online {
  background: rgba(34, 197, 94, 0.1);
  color: #16a34a;
}

.status-badge.offline {
  background: var(--hover-bg);
  color: var(--text-secondary);
}

html.dark .status-badge.online {
  background: rgba(34, 197, 94, 0.15);
  color: #4ade80;
}

.header-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  font-size: 13px;
  color: var(--text-secondary);
}

.meta-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--text-secondary);
  text-decoration: none;
  transition: color 0.2s;
}

.meta-link:hover {
  color: var(--el-color-primary);
}

.meta-text {
  color: var(--text-muted);
}

.meta-sep {
  margin: 0 4px;
}

/* Stats Bar */
.stats-bar {
  display: flex;
  background: var(--el-bg-color);
  border: 1px solid var(--header-border);
  border-radius: 10px;
  margin-bottom: 20px;
}

.stats-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 20px;
}

.stats-item + .stats-item {
  border-left: 1px solid var(--header-border);
}

.stats-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.stats-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.stats-value small {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
}

.conn-link {
  cursor: pointer;
  color: var(--el-color-primary);
}

.conn-link:hover {
  text-decoration: underline;
}


/* Card Base */
.traffic-card {
  background: var(--el-bg-color);
  border: 1px solid var(--header-border);
  border-radius: 12px;
  margin-bottom: 16px;
}

/* Config Section */
.config-section {
  margin-bottom: 24px;
}

.config-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  color: var(--text-secondary);
}

.config-section-header h2 {
  font-size: 16px;
  font-weight: 500;
  color: var(--text-primary);
  margin: 0;
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.config-item-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 20px;
  background: var(--el-bg-color);
  border: 1px solid var(--header-border);
  border-radius: 12px;
}

.config-item-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.config-item-icon.encryption {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.config-item-icon.compression {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.config-item-icon.domains {
  background: rgba(168, 85, 247, 0.1);
  color: #a855f7;
}

.config-item-icon.subdomain {
  background: rgba(168, 85, 247, 0.1);
  color: #a855f7;
}

.config-item-icon.locations {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.config-item-icon.host {
  background: rgba(249, 115, 22, 0.1);
  color: #f97316;
}

.config-item-icon.multiplexer {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.config-item-icon.route {
  background: rgba(236, 72, 153, 0.1);
  color: #ec4899;
}

.config-item-icon.allow-ip {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.config-item-icon.deny-ip {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.config-item-icon.useragent {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.config-item-icon.backend {
  background: rgba(234, 179, 8, 0.1);
  color: #ca8a04;
}

html.dark .config-item-icon.backend {
  background: rgba(234, 179, 8, 0.15);
  color: #facc15;
}

html.dark .config-item-icon.allow-ip {
  background: rgba(34, 197, 94, 0.15);
}

html.dark .config-item-icon.deny-ip {
  background: rgba(239, 68, 68, 0.15);
}

html.dark .config-item-icon.useragent {
  background: rgba(59, 130, 246, 0.15);
}

.config-tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}

.config-tag {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  font-family: monospace;
}

.config-tag--allow {
  background: rgba(34, 197, 94, 0.1);
  color: #16a34a;
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.config-tag--deny {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
  border: 1px solid rgba(239, 68, 68, 0.2);
}

.config-tag--ua {
  background: rgba(59, 130, 246, 0.1);
  color: #2563eb;
  border: 1px solid rgba(59, 130, 246, 0.2);
}

html.dark .config-tag--allow {
  background: rgba(34, 197, 94, 0.15);
  color: #4ade80;
  border-color: rgba(34, 197, 94, 0.25);
}

html.dark .config-tag--deny {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  border-color: rgba(239, 68, 68, 0.25);
}

html.dark .config-tag--ua {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
  border-color: rgba(59, 130, 246, 0.25);
}

html.dark .config-item-icon.encryption,
html.dark .config-item-icon.compression {
  background: rgba(34, 197, 94, 0.15);
}

html.dark .config-item-icon.domains,
html.dark .config-item-icon.subdomain {
  background: rgba(168, 85, 247, 0.15);
}

html.dark .config-item-icon.locations,
html.dark .config-item-icon.multiplexer {
  background: rgba(59, 130, 246, 0.15);
}

html.dark .config-item-icon.host {
  background: rgba(249, 115, 22, 0.15);
}

html.dark .config-item-icon.route {
  background: rgba(236, 72, 153, 0.15);
}
.config-item-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.config-item-label {
  font-size: 13px;
  color: var(--text-secondary);
  font-weight: 500;
}

.config-item-value {
  font-size: 15px;
  color: var(--text-primary);
  font-weight: 500;
  word-break: break-all;
}

.annotations-section {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}

.annotation-tag {
  display: inline-flex;
  padding: 6px 12px;
  background: var(--el-fill-color);
  border-radius: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  font-weight: 500;
}

.traffic-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--header-border);
}

.traffic-header h2 {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-primary);
  margin: 0;
}

/* Traffic Card */
.traffic-body {
  padding: 20px;
}

/* Not Found */
.not-found {
  text-align: center;
  padding: 60px 20px;
}

.not-found h2 {
  font-size: 18px;
  font-weight: 500;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.not-found p {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0 0 20px;
}

/* Responsive */
@media (max-width: 768px) {
  .config-grid {
    grid-template-columns: 1fr;
  }

  .stats-bar {
    flex-wrap: wrap;
  }

  .stats-item {
    flex: 1 1 40%;
  }

  .stats-item:nth-child(n+3) {
    border-top: 1px solid var(--header-border);
  }
}

@media (max-width: 640px) {
  .header-main {
    flex-direction: column;
    gap: 16px;
  }

}

/* Access Log */
.access-log-card {
  background: var(--el-bg-color);
  border: 1px solid var(--header-border);
  border-radius: 12px;
  margin-bottom: 16px;
}

.access-log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--header-border);
  flex-wrap: wrap;
  gap: 12px;
}

.access-log-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
}

.access-log-title h2 {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-primary);
  margin: 0;
}

.access-log-filters {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.access-log-body {
  padding: 16px 20px;
}

.access-log-pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.log-ip {
  color: var(--el-color-primary);
  cursor: pointer;
  font-family: monospace;
}

.log-ip:hover {
  text-decoration: underline;
}

.log-port {
  color: var(--text-secondary);
  font-family: monospace;
  font-size: 12px;
}

.log-host {
  font-family: monospace;
  font-size: 12px;
  color: var(--text-primary);
}

.log-ua {
  font-size: 12px;
  color: var(--text-secondary);
}

.log-empty {
  color: var(--text-muted, var(--text-secondary));
}

.log-ok {
  color: var(--text-secondary);
}

.traffic-in {
  color: #22c55e;
  font-size: 12px;
  font-family: monospace;
}

.traffic-sep {
  color: var(--text-secondary);
}

.traffic-out {
  color: #3b82f6;
  font-size: 12px;
  font-family: monospace;
}
</style>
