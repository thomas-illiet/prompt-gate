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
  () => `package main

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	client := openai.NewClient(
		option.WithAPIKey("${PROMPTGATE_TOKEN_PLACEHOLDER}"),
		option.WithBaseURL("${baseUrl.value}"),
	)

	chatCompletion, err := client.Chat.Completions.New(
		context.Background(),
		openai.ChatCompletionNewParams{
			Model: openai.ChatModel("${effectiveModel.value}"),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello from PromptGate"),
			},
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(chatCompletion.Choices[0].Message.Content)
}`,
)

const subtitle = 'Official OpenAI Go SDK.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="Go" />
</template>
