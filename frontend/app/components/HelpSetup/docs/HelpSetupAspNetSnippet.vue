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
  () => `using System.Net.Http.Headers;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddHttpClient();

var app = builder.Build();

app.MapPost("/chat", async (IHttpClientFactory httpClientFactory) =>
{
    using var client = httpClientFactory.CreateClient();
    client.DefaultRequestHeaders.Authorization =
        new AuthenticationHeaderValue("Bearer", "${PROMPTGATE_TOKEN_PLACEHOLDER}");

    var response = await client.PostAsJsonAsync(
        "${baseUrl.value}/chat/completions",
        new
        {
            model = "${effectiveModel.value}",
            messages = new[]
            {
                new { role = "user", content = "Hello from PromptGate" },
            },
        });

    response.EnsureSuccessStatusCode();
    return Results.Content(
        await response.Content.ReadAsStringAsync(),
        "application/json");
});

app.Run();`,
)

const subtitle = 'ASP.NET minimal API proxying a chat completion request.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="ASP.NET" />
</template>
