<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Sim24Regular } from '@vicons/fluent'
import { Loading } from '@element-plus/icons-vue'
import type { CardPolicy } from '../types/api'
import { devicesService } from '../services/devices'

const props = defineProps<{
  deviceId: string | undefined
  iccid: string | undefined
  policy: CardPolicy | null
  deviceOnline: boolean
}>()

const emit = defineEmits<{
  policyChanged: []
}>()

// 本地镜像（跟上游 policy 同步）。airplane 直接镜像存储的“用户飞行意图”。
const local = ref<{
  network_enabled: boolean
  airplane_enabled: boolean
  ip_version: 'v4' | 'v6' | 'v4v6'
  apn: string
}>({ network_enabled: false, airplane_enabled: false, ip_version: 'v4', apn: '' })

// 各开关的热切换中间态（pending/failed）
const networkPending = ref(false)
const networkFailed = ref(false)
const airplanePending = ref(false)
const airplaneFailed = ref(false)

// 上游 policy 变化时原地同步各字段（不整体替换对象，避免 el-switch 崩溃）
watch(
  () => props.policy,
  (p) => {
    if (!p) return
    local.value.network_enabled = p.network_enabled
    local.value.airplane_enabled = p.airplane_enabled
    local.value.ip_version = p.ip_version || 'v4'
    local.value.apn = p.apn || ''
    networkFailed.value = false
    airplaneFailed.value = false
  },
  { immediate: true }
)

const sourceLabel = computed(() => {
  if (!props.policy) return ''
  return props.policy.source === 'user' ? '手动设置' : '自动默认'
})

const canToggle = computed(() => props.deviceOnline && !!props.iccid)

async function onNetworkToggle(rawVal: string | number | boolean) {
  const val = rawVal as boolean
  if (!props.deviceId || !canToggle.value) return
  networkPending.value = true
  networkFailed.value = false
  const prev = !val
  let result
  if (val) {
    result = await devicesService.startNetwork(props.deviceId, {
      ip_version: local.value.ip_version,
      apn: local.value.apn
    })
  } else {
    result = await devicesService.stopNetwork(props.deviceId)
  }
  networkPending.value = false
  if (!result.ok) {
    local.value.network_enabled = prev
    networkFailed.value = true
  } else {
    networkFailed.value = false
    // 开网络与飞行互斥（后端已互斥落库，这里同步 UI）
    if (val) {
      local.value.airplane_enabled = false
    }
    emit('policyChanged')
  }
}

async function onAirplaneToggle(rawVal: string | number | boolean) {
  const val = rawVal as boolean
  if (!props.deviceId || !canToggle.value) return
  airplanePending.value = true
  airplaneFailed.value = false
  const prev = !val
  const result = await devicesService.setFlightMode(props.deviceId, val)
  airplanePending.value = false
  if (!result.ok) {
    local.value.airplane_enabled = prev
    airplaneFailed.value = true
  } else {
    airplaneFailed.value = false
    // 开飞行与网络互斥（后端已互斥落库，这里同步 UI）
    if (val) {
      local.value.network_enabled = false
    }
    emit('policyChanged')
  }
}
</script>

<template>
  <div>
    <!-- 标题行 -->
    <div class="flex items-center gap-3 mb-4">
      <div class="w-10 h-10 rounded-xl bg-violet-50 dark:bg-violet-500/10 flex items-center justify-center text-violet-600 dark:text-violet-400">
        <el-icon size="22"><Sim24Regular /></el-icon>
      </div>
      <div>
        <div class="text-lg font-bold text-gray-900 dark:text-white">卡策略</div>
        <div class="text-xs text-gray-500 dark:text-gray-400">网络/飞行模式开关跟着 SIM 卡走，切换即时生效</div>
      </div>
    </div>

    <!-- 无 ICCID 提示 -->
    <div v-show="!iccid" class="ui-panel-muted p-4 text-center text-sm text-gray-500 dark:text-gray-400">
      设备尚未识别到 SIM 卡 ICCID，策略不可操作
    </div>

    <!-- 离线提示（有 ICCID 但设备离线） -->
    <div v-show="iccid && !deviceOnline" class="mb-3 px-3 py-2 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 text-xs text-yellow-700 dark:text-yellow-300">
      设备离线，策略仅展示，切换操作已禁用
    </div>

    <!-- 用 v-show 让 el-switch 始终挂载，避免 element-plus 2.13 在挂载前访问未就绪 input 而崩溃 -->
    <div v-show="iccid" class="space-y-3">
      <!-- ICCID + 来源 -->
      <div class="ui-panel-muted p-3 flex items-center justify-between">
        <div>
          <div class="text-xs font-bold text-gray-500 uppercase tracking-wider mb-0.5">当前卡 ICCID</div>
          <div class="text-sm font-mono text-gray-800 dark:text-gray-100">{{ iccid }}</div>
        </div>
        <el-tag v-if="sourceLabel" :type="policy?.source === 'user' ? 'primary' : 'info'" size="small">{{ sourceLabel }}</el-tag>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
                <!-- IP 版本 -->
        <div class="space-y-1">
          <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">IP 版本</label>
          <el-select v-model="local.ip_version" class="w-full" :disabled="!canToggle">
            <el-option label="IPv4" value="v4" />
            <el-option label="IPv6" value="v6" />
            <el-option label="IPv4 + IPv6（双栈）" value="v4v6" />
          </el-select>
          <div class="text-xs text-gray-400">下次开启网络时生效</div>
        </div>

        <!-- APN -->
        <div class="space-y-1">
          <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">APN（可选）</label>
          <el-input v-model="local.apn" placeholder="留空自动识别" :disabled="!canToggle" />
          <div class="text-xs text-gray-400">下次开启网络时生效</div>
        </div>
        <!-- 开启网络 -->
        <div
          class="ui-panel-muted p-3 space-y-1"
          :class="local.network_enabled ? 'border border-emerald-300 bg-emerald-50/50 dark:bg-emerald-900/20' : ''"
        >
          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-bold text-gray-800 dark:text-gray-100">开启网络</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">飞行模式开启时不可用</div>
            </div>
            <div class="flex items-center gap-2">
              <span v-if="networkFailed" class="text-xs text-orange-500 dark:text-orange-400">未生效</span>
              <el-icon v-if="networkPending" class="animate-spin text-gray-400"><Loading /></el-icon>
              <el-switch
                v-model="local.network_enabled"
                :disabled="!canToggle || local.airplane_enabled || networkPending"
                @change="onNetworkToggle"
              />
            </div>
          </div>
        </div>

        <!-- 飞行模式 -->
        <div
          class="ui-panel-muted p-3 space-y-1"
          :class="local.airplane_enabled ? 'border border-sky-300 bg-sky-50/50 dark:bg-sky-900/20' : ''"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <div>
                <div class="text-sm font-bold text-gray-800 dark:text-gray-100">飞行模式</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">射频关闭，断网</div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span v-if="airplaneFailed" class="text-xs text-orange-500 dark:text-orange-400">未生效</span>
              <el-icon v-if="airplanePending" class="animate-spin text-gray-400"><Loading /></el-icon>
              <el-switch
                v-model="local.airplane_enabled"
                :disabled="!canToggle || airplanePending"
                @change="onAirplaneToggle"
              />
            </div>
          </div>
        </div>


      </div>
    </div>
  </div>
</template>
