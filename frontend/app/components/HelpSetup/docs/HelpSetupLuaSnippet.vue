<script setup lang="ts">
import HelpSetupSnippetCard from '~/components/HelpSetup/HelpSetupSnippetCard.vue'
import type { HelpSetupProvider } from '~/types/user-service'
import {
  PROMPTGATE_TOKEN_PLACEHOLDER,
  firstUsableModel,
} from '~/utils/help-setup'

const props = defineProps<{
  model: string
  provider: HelpSetupProvider
}>()

const effectiveModel = computed(
  () => props.model || firstUsableModel(props.provider),
)
const baseUrl = computed(() => props.provider.openaiBaseUrl ?? '')
const code = computed(
  () => `local https = require("ssl.https")
local ltn12 = require("ltn12")
local json = require("dkjson")

local body = json.encode({
  model = "${effectiveModel.value}",
  messages = {
    { role = "user", content = "Hello from PromptGate" },
  },
})

local response = {}
local ok, status = https.request({
  url = "${baseUrl.value}/chat/completions",
  method = "POST",
  headers = {
    ["Authorization"] = "Bearer ${PROMPTGATE_TOKEN_PLACEHOLDER}",
    ["Content-Type"] = "application/json",
    ["Content-Length"] = tostring(#body),
  },
  source = ltn12.source.string(body),
  sink = ltn12.sink.table(response),
})

if not ok then
  error(status)
end

local payload = json.decode(table.concat(response))
print(payload.choices[1].message.content)`,
)

const subtitle = 'Lua request with luasocket, LuaSec, and dkjson.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="Lua" />
</template>
