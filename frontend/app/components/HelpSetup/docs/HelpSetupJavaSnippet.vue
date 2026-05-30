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
  () => `import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;

public class PromptGateChat {
    public static void main(String[] args) throws Exception {
        String body = """
            {
              "model": "${effectiveModel.value}",
              "messages": [
                {
                  "role": "user",
                  "content": "Hello from PromptGate"
                }
              ]
            }
            """;

        HttpRequest request = HttpRequest.newBuilder()
            .uri(URI.create("${baseUrl.value}/chat/completions"))
            .header("Authorization", "Bearer ${PROMPTGATE_TOKEN_PLACEHOLDER}")
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(body))
            .build();

        HttpResponse<String> response = HttpClient.newHttpClient().send(
            request,
            HttpResponse.BodyHandlers.ofString());

        System.out.println(response.body());
    }
}`,
)

const subtitle = 'Java HTTP client request.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="Java" />
</template>
