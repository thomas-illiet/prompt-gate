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
  () => `$baseUrl = "${baseUrl.value}"
$headers = @{
  "Authorization" = "Bearer ${PROMPTGATE_TOKEN_PLACEHOLDER}"
  "Content-Type" = "application/json"
}
$body = @{
  model = "${effectiveModel.value}"
  messages = @(
    @{ role = "user"; content = "Hello from PromptGate" }
  )
} | ConvertTo-Json -Depth 5

$response = Invoke-RestMethod \`
  -Uri "$baseUrl/chat/completions" \`
  -Method Post \`
  -Headers $headers \`
  -Body $body

$response.choices[0].message.content`,
)

const subtitle = 'PowerShell request with Invoke-RestMethod.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="PowerShell" />
</template>
