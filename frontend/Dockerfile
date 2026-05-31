# syntax=docker/dockerfile:1.7

# The deps stage installs an exact dependency tree from package-lock.json.
# Keeping this in a dedicated stage lets Docker reuse npm ci when only source
# files change.
FROM node:24-alpine AS deps

WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN npm ci

# The builder stage compiles the Nuxt app into Nitro output.
# This project runs as a client-side Nuxt app, but nuxt build still produces a
# small Node server in .output that serves the production assets.
FROM deps AS builder

COPY . .
RUN npm run build

# The runtime stage contains only the production Nuxt output.
# Dependencies and source files from the builder are intentionally left behind.
FROM node:24-alpine AS runtime

WORKDIR /app

# Nitro must listen on all interfaces inside the container, and the documented
# service port for this image is 3000.
ENV NODE_ENV=production
ENV NITRO_HOST=0.0.0.0
ENV NITRO_PORT=3000

# Run the application as a fixed non-root user. UID/GID 1000 keeps file
# ownership predictable for common Linux hosts and Kubernetes securityContext
# settings. The official Node Alpine image already reserves UID/GID 1000 for
# the default node account, so the runtime stage removes that account first and
# recreates the identity with the required appuser name.
RUN if grep -q '^node:' /etc/passwd; then deluser node; fi \
  && if grep -q '^node:' /etc/group; then delgroup node; fi \
  && addgroup -g 1000 -S appuser \
  && adduser -u 1000 -S appuser -G appuser

# Copy only the production output and assign it to the runtime user so the Node
# process never needs root privileges.
COPY --from=builder --chown=appuser:appuser /app/.output ./.output

USER appuser

EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
