<template>
  <div class="acl-page">
    <!-- Page header -->
    <div class="page-header">
      <div>
        <h1 class="page-title">Access Control</h1>
        <p class="page-subtitle">
          Global IP access control rules — applied to all proxy types (TCP, UDP, HTTP, etc.) before per-proxy rules
        </p>
      </div>
      <el-button
        type="primary"
        :loading="saving"
        :disabled="loading"
        @click="handleSave"
      >
        Save
      </el-button>
    </div>

    <div v-loading="loading" class="acl-content">
      <!-- Allow IPs -->
      <el-card shadow="hover" class="acl-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">Allow IPs</span>
            <el-tag size="small" type="success">Whitelist</el-tag>
          </div>
        </template>
        <p class="field-desc">
          Only requests from these IPs/CIDRs are allowed (when non-empty). Leave
          empty to allow all IPs.
        </p>
        <TagListEditor
          v-model="form.allowIPs"
          placeholder="e.g. 192.168.1.0/24"
          :validator="validateIPOrCIDR"
          validator-message="Invalid IP or CIDR format"
        />
      </el-card>

      <!-- Deny IPs -->
      <el-card shadow="hover" class="acl-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">Deny IPs</span>
            <el-tag size="small" type="danger">Blacklist · highest priority</el-tag>
          </div>
        </template>
        <p class="field-desc">
          Requests from these IPs/CIDRs are always rejected, regardless of allow
          rules.
        </p>
        <TagListEditor
          v-model="form.denyIPs"
          placeholder="e.g. 10.0.0.100"
          :validator="validateIPOrCIDR"
          validator-message="Invalid IP or CIDR format"
        />
      </el-card>

      <!-- Allow User-Agents -->
      <el-card shadow="hover" class="acl-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">Allow User-Agents</span>
            <el-tag size="small" type="success">Whitelist · supports * wildcard</el-tag>
          </div>
        </template>
        <p class="field-desc">
          Requests matching any of these User-Agent glob patterns are allowed (OR
          logic with Allow IPs). Leave empty to skip UA check.
        </p>
        <TagListEditor
          v-model="form.allowUserAgents"
          placeholder="e.g. Mozilla/*"
        />
      </el-card>

      <!-- Deny User-Agents -->
      <el-card shadow="hover" class="acl-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">Deny User-Agents</span>
            <el-tag size="small" type="danger">Blacklist · supports * wildcard</el-tag>
          </div>
        </template>
        <p class="field-desc">
          Requests matching any of these User-Agent glob patterns are always
          rejected.
        </p>
        <TagListEditor
          v-model="form.denyUserAgents"
          placeholder="e.g. *bot*"
        />
      </el-card>

      <!-- Rule priority note -->
      <el-card shadow="hover" class="acl-card hint-card">
        <div class="priority-hint">
          <el-icon class="hint-icon"><InfoFilled /></el-icon>
          <div>
            <div class="hint-title">Rule evaluation order (applies to all proxy types)</div>
            <ol class="hint-list">
              <li>Global Deny IPs — always rejects matching IPs across all proxies</li>
              <li>Global Allow IPs — whitelist; empty means no global restriction</li>
              <li>Per-proxy Deny IPs — rejects for this proxy</li>
              <li>Per-proxy Allow IPs — whitelist for this proxy</li>
              <li>HTTP/HTTPS only: Deny/Allow User-Agents (per-proxy, evaluated after IP rules)</li>
            </ol>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import { getGlobalACL, updateGlobalACL } from '../api/acl'
import TagListEditor from '../components/TagListEditor.vue'

const loading = ref(false)
const saving = ref(false)

const form = reactive({
  allowIPs: [] as string[],
  denyIPs: [] as string[],
  allowUserAgents: [] as string[],
  denyUserAgents: [] as string[],
})

/** Validates an IPv4/IPv6 address or CIDR notation. */
function validateIPOrCIDR(value: string): boolean {
  const cidrRegex = /^[0-9a-fA-F.:]+\/\d{1,3}$/
  if (cidrRegex.test(value)) {
    const [base, prefixStr] = value.split('/')
    const prefix = parseInt(prefixStr, 10)
    // IPv6 CIDR
    if (base.includes(':')) return prefix >= 0 && prefix <= 128
    // IPv4 CIDR
    if (prefix < 0 || prefix > 32) return false
    return (
      /^(\d{1,3}\.){3}\d{1,3}$/.test(base) &&
      base.split('.').every((o) => parseInt(o, 10) <= 255)
    )
  }
  // Pure IPv4
  if (/^(\d{1,3}\.){3}\d{1,3}$/.test(value)) {
    return value.split('.').every((o) => parseInt(o, 10) <= 255)
  }
  // Pure IPv6
  return /^[0-9a-fA-F:]+$/.test(value) && value.includes(':')
}

async function fetchACL() {
  loading.value = true
  try {
    const data = await getGlobalACL()
    form.allowIPs = data.allowIPs ?? []
    form.denyIPs = data.denyIPs ?? []
    form.allowUserAgents = data.allowUserAgents ?? []
    form.denyUserAgents = data.denyUserAgents ?? []
  } catch {
    ElMessage({ type: 'error', message: 'Failed to load ACL config', showClose: true })
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const resp = await updateGlobalACL({
      allowIPs: form.allowIPs,
      denyIPs: form.denyIPs,
      allowUserAgents: form.allowUserAgents,
      denyUserAgents: form.denyUserAgents,
    })
    if (resp.persisted) {
      ElMessage({ type: 'success', message: 'ACL rules saved and persisted to config file', showClose: true })
    } else {
      ElMessage({
        type: 'warning',
        message: 'ACL rules are active, but could not be written to config file — changes will be lost after restart',
        showClose: true,
        duration: 6000,
      })
    }
  } catch (err: any) {
    const msg = err?.message || 'Failed to save ACL rules'
    ElMessage({ type: 'error', message: msg, showClose: true })
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchACL()
})
</script>

<style scoped>
.acl-page {
  padding: 0;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 24px;
  gap: 16px;
}

.acl-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 200px;
}

.acl-card {
  border-radius: 12px;
  border: 1px solid #e4e7ed;
}

html.dark .acl-card {
  border-color: #3a3d5c;
  background: #27293d;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 15px;
  font-weight: 500;
  color: #303133;
}

html.dark .card-title {
  color: #e5e7eb;
}

.field-desc {
  font-size: 13px;
  color: #909399;
  margin: 0 0 12px;
  line-height: 1.5;
}

/* Hint card */
.hint-card {
  background: #f8f9fa;
}

html.dark .hint-card {
  background: #1e1e2d;
}

.priority-hint {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.hint-icon {
  font-size: 18px;
  color: #909399;
  flex-shrink: 0;
  margin-top: 2px;
}

.hint-title {
  font-size: 14px;
  font-weight: 500;
  color: #606266;
  margin-bottom: 8px;
}

html.dark .hint-title {
  color: #b0b0b0;
}

.hint-list {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: #909399;
  line-height: 1.8;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .page-header .el-button {
    width: 100%;
  }
}
</style>
