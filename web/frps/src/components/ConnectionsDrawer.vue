<template>
  <el-drawer v-model="visible" title="Connection History" size="75%" direction="rtl" @open="fetchData">
    <div class="drawer-body">
      <div class="filter-row">
        <el-input
          v-model="filterIP"
          placeholder="Filter by IP"
          clearable
          style="width: 180px"
          @change="onFilterChange"
        />
        <el-input
          v-model="filterProxy"
          placeholder="Filter by proxy"
          clearable
          style="width: 180px"
          @change="onFilterChange"
        />
        <el-select
          v-model="filterEvent"
          style="width: 130px"
          @change="onFilterChange"
        >
          <el-option label="All Events" value="" />
          <el-option label="Active" value="connected" />
          <el-option label="Closed" value="disconnected" />
          <el-option label="Blocked" value="blocked" />
        </el-select>
        <el-button :loading="loading" @click="fetchData">Refresh</el-button>
      </div>

      <el-table :data="records" v-loading="loading" stripe size="small" style="width: 100%">
        <el-table-column label="Event" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.event === 'connected'" type="primary" size="small">Active</el-tag>
            <el-tag v-else-if="row.event === 'disconnected'" type="info" size="small">Closed</el-tag>
            <el-tag v-else-if="row.event === 'blocked'" type="danger" size="small">Blocked</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="Remote" width="160">
          <template #default="{ row }">{{ row.remoteIP }}:{{ row.remotePort }}</template>
        </el-table-column>
        <el-table-column prop="proxyName" label="Proxy" min-width="120" show-overflow-tooltip />
        <el-table-column label="Connected At" width="170">
          <template #default="{ row }">{{ formatTime(row.connectedAt) }}</template>
        </el-table-column>
        <el-table-column label="Duration" width="100" align="right">
          <template #default="{ row }">
            {{ row.duration ? formatDuration(row.duration) : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Traffic In" width="100" align="right">
          <template #default="{ row }">
            {{ row.trafficIn ? formatFileSize(row.trafficIn) : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Traffic Out" width="100" align="right">
          <template #default="{ row }">
            {{ row.trafficOut ? formatFileSize(row.trafficOut) : '-' }}
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @current-change="fetchData"
          @size-change="onSizeChange"
        />
      </div>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { formatFileSize } from '../utils/format'
import { getAccessLog } from '../api/proxy'
import type { AccessLogRecord } from '../types/proxy'

const props = defineProps<{ modelValue: boolean; proxyName?: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()

const visible = ref(false)
watch(
  () => props.modelValue,
  (v) => (visible.value = v),
)
watch(visible, (v) => {
  if (v && props.proxyName) {
    filterProxy.value = props.proxyName
  }
  emit('update:modelValue', v)
})

const loading = ref(false)
const records = ref<AccessLogRecord[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filterIP = ref('')
const filterProxy = ref('')
const filterEvent = ref('')

const fetchData = async () => {
  loading.value = true
  try {
    const result = await getAccessLog({
      remoteIP: filterIP.value || undefined,
      proxyName: filterProxy.value || undefined,
      event: filterEvent.value || undefined,
      page: page.value,
      pageSize: pageSize.value,
    })
    records.value = result.records ?? []
    total.value = result.total
  } catch {
    ElMessage({ type: 'error', message: 'Failed to load connection history' })
  } finally {
    loading.value = false
  }
}

const onFilterChange = () => {
  page.value = 1
  fetchData()
}

const onSizeChange = () => {
  page.value = 1
  fetchData()
}

const formatTime = (ms: number) => new Date(ms).toLocaleString()

const formatDuration = (ms: number) => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}
</script>

<style scoped>
.drawer-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: 100%;
}

.filter-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  padding-top: 8px;
}
</style>
