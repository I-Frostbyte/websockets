build:
	docker build -f ./Dockerfile -t $(full_image_tag) ..

publish:
	docker push $(full_image_tag)

rundb:
	docker run --name mqtlocal -it -p 1883:1883 -p 9001:9001 -p 8883:8883 -d eclipse-mosquitto:2

createdockercontainer:
	docker run -it -d \
	--name mqtlocal \
	-p 1883:1883 \
	-p 9001:9001 \
	-p 8883:8883 \
	-v "/mnt/c/Program Files/mosquitto/mosquitto.conf:/mosquitto/config/mosquitto.conf" \
	eclipse-mosquitto:2

run:
	mosquitto -c mosquitto.conf -y
