export default defineNuxtPlugin(async () => {
  await useInstanceStore().initialize()
})
