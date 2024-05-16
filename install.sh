#!/bin/bash

sudo dnf install golang

cd src
go build .
sudo cp mqtt_topic_frequenzy_counter /usr/local/bin/ -v
cd ..
sudo cp ./systemd/mqtt_topic_frequenzy_counter.service /etc/systemd/user/
sudo systemctl enable mqtt_topic_frequenzy_counter.service