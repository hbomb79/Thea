FROM node:16-alpine
EXPOSE 5000
WORKDIR /app

COPY . .

RUN npm install

CMD [ "npm", "run", "dev" ]