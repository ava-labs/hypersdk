import {useEffect, useState} from "react";
import { App, Divider, List, Card, Typography, Form, Input, InputNumber, Button, Select, Spin } from "antd";
import { CheckCircleTwoTone, CloseCircleTwoTone, LoadingOutlined } from '@ant-design/icons';
import { StartFaucetSearch, GetFaucetSolutions } from "../../wailsjs/go/main/App";
const { Text, Title, Link } = Typography;

const Trade = () => {
    const { message } = App.useApp();

    useEffect(() => {
      const interval = setInterval(() => {
      }, 500);

      return () => clearInterval(interval);
    }, []);

    return (<>
            <div style={{ width:"60%", margin: "auto" }}>
      <Divider orientation="center" >
        Open Orders
      </Divider>
      <List
        bordered
        dataSource={[]}
        renderItem={(item) => (
          <List.Item>
            <div>
              <Text strong>{item.Solution} </Text>
              <CloseCircleTwoTone twoToneColor="#eb2f96" />
            </div>
            <Text strong>Salt:</Text> {item.Salt}
            <br />
            <Text strong>Difficulty:</Text> {item.Difficulty}
            <br />
            <Text strong>Attempts:</Text> {item.Attempts}
            <br />
            <Text strong>Elapsed:</Text> {item.Elapsed}
            <br />
            <Text strong>Error:</Text> {item.Err}
          </List.Item>
        )}
      />
      <Divider orientation="center" >
        Order Book
      </Divider>
      <List
        bordered
        dataSource={[]}
        renderItem={(item) => (
          <List.Item>
            <div>
              <Text strong>{item.Solution} </Text>
              <CloseCircleTwoTone twoToneColor="#eb2f96" />
            </div>
            <Text strong>Salt:</Text> {item.Salt}
            <br />
            <Text strong>Difficulty:</Text> {item.Difficulty}
            <br />
            <Text strong>Attempts:</Text> {item.Attempts}
            <br />
            <Text strong>Elapsed:</Text> {item.Elapsed}
            <br />
            <Text strong>Error:</Text> {item.Err}
          </List.Item>
        )}
      />
      </div>
    </>);
}
export default Trade;
