FROM node:22-alpine

# Install git (required by git-cache-http-server)
RUN apk add --no-cache git

# Install the server globally
RUN npm install --global git-cache-http-server

# Create a non‑root user with UID 1001 (since 1000 is taken by 'node')
RUN adduser -D -u 1001 appuser

# Create cache directory and set ownership
RUN mkdir -p /var/cache/git && chown -R appuser:appuser /var/cache/git

USER appuser

EXPOSE 8080

ENTRYPOINT ["git-cache-http-server"]
CMD ["--port", "8080", "--cache-dir", "/var/cache/git"]