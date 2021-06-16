FROM amazon/aws-lambda-nodejs:14

WORKDIR /app

ENV AWS_LAMBDA=true

# We need node_modules in our docker container
COPY package.json package-lock.json ./
RUN npm install

COPY ./ ./

RUN npm run compile

CMD [ "/app/build/src/handlers/index.handler" ]