FROM node:25-alpine AS builder

WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

ARG VITE_ENABLE_GOOGLE_SIGNIN
ENV VITE_ENABLE_GOOGLE_SIGNIN=$VITE_ENABLE_GOOGLE_SIGNIN

ARG VITE_API_BASE_URL
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

RUN npm run build

FROM nginx:stable-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /build/dist /usr/share/nginx/html
