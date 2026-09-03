# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# Use mainland mirrors for Alpine and Node package downloads.
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install bash git dos2unix for TypeScript compiler
RUN apk add --no-cache bash git dos2unix

ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
RUN npm config set registry "$NPM_CONFIG_REGISTRY"

# Copy rush.json from root directory first
COPY rush.json ./

# Install Rush from the configured mainland npm mirror.
RUN npm install -g @microsoft/rush --registry="$NPM_CONFIG_REGISTRY"

# Copy the frontend source code (excluding node_modules and build artifacts)
COPY frontend/ ./frontend/

# Copy common directory for Rush
COPY common/ ./common/

# Copy scripts directory for Rush hooks
COPY scripts/ ./scripts/

# Convert line endings for shell scripts in scripts directory to Unix format
RUN find . -name "*.sh" -type f -exec dos2unix {} \;

# Install all dependencies
RUN chmod +x scripts/hooks/post-rush-install.sh && rm -rf /app/common/temp && rush install

# Use rush build to build the specific project
RUN rush build --to @coze-studio/app

# Production image stage
FROM nginx:1.25-alpine


# Use the same mainland mirror for runtime Alpine packages.
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
# Install necessary tools for SSL configuration
RUN apk add --no-cache bash

# Copy build artifacts to nginx static directory
COPY --from=builder /app/frontend/apps/coze-studio/dist /usr/share/nginx/html

# Copy the Nginx configuration into the image so runtime does not need host mounts.
COPY docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY docker/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf

EXPOSE 8888

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
