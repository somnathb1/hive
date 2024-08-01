import json

from mitmproxy import http
from time import sleep
import os
import configparser
import urllib.request

# Read the configuration file
config = configparser.ConfigParser()
config.read(f'/home/mitmproxy/.mitmproxy/{os.getenv("MITMPROXY_CONFIG_NAME", "config")}.ini')
container_id = None
print("Config is:", os.getenv("MITMPROXY_CONFIG_NAME", "config"))
print("Experiment ID is:", os.getenv("MITMPROXY_EXPERIMENT_ID", "debug"))

def request(flow: http.HTTPFlow) -> None:
    print(f"Sending request to: {flow.request.url}")
    test_end = config['settings'].get('test_end', None)
    post_url = config['settings']['target_url']
    copy_files = config['settings']['copy_files']
    experiment_id = os.getenv("MITMPROXY_EXPERIMENT_ID", "debug")
    if flow.request.method == "POST" and flow.request.path == test_end:
        sleep(10)
        print("Starting copying files ...")
        if not copy_files or copy_files == "False":
            print("Copying files is disabled ...")
            return
        for key in config['copy_files']:

            if key.startswith('container_file_path_'):
                print("Valid config is found for experiment: " + experiment_id)
                file_number = key.split('_')[-1]
                container_file_path = config['copy_files'][key]
                file_name = container_file_path.split('/')[-1]
                host_file_path = config['copy_files'][f'host_file_path_{file_number}'] + "/" + experiment_id + "/" + file_name
                # Create the POST request
                data = {
                    "container_id": container_id,
                    "container_file_path": container_file_path,
                    "host_file_path": host_file_path
                }
                # Convert the dictionary to a JSON string
                json_data = json.dumps(data).encode('utf-8')  # Encode to bytes
                print(json_data)
                try:
                    # Send the POST request using urllib3
                    headers = {"Content-Type": "application/json"}
                    req = urllib.request.Request(
                        post_url + "/copyfile",
                        data=json_data, headers=headers
                    )

                    with urllib.request.urlopen(req) as rp:
                        print(f"Sent POST request to {post_url} for file {container_file_path} to {host_file_path}")
                        print(f"Response status: {rp.status}")
                except Exception as e:
                    print(f"Error: {e} occurred while sending the POST request to {post_url}")
                sleep(config['settings'].getint('end_wait_time', 30))
    # Drop the request
    # flow.kill()

def response(flow: http.HTTPFlow) -> None:
    global container_id
    # This function is called when a response is received
    # print(f"Response received for: {flow.request.pretty_url}")

    if "/node" in flow.request.path:
        if flow.response:
            print(f"Response status code: {flow.response.status_code}")
            if flow.response.status_code == 200:
                try:
                    print(f"Response::: {flow.response.get_text()}")
                    container_id = json.loads(flow.response.get_text())['id']
                    print(f"Container ID: {container_id}")
                except json.JSONDecodeError:
                    print("Response is not valid JSON.")
            else:
                print(f"Error: Received status code {flow.response.status_code}")

