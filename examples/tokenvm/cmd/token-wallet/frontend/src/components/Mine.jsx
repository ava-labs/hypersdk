import {useEffect, useState} from "react";
import { Card, Typography, Form, Input, InputNumber, Button, Select, Spin, message } from "antd";
import { LoadingOutlined } from '@ant-design/icons';
import { GetBalance } from "../../wailsjs/go/main/App";
const { Text, Title, Link } = Typography;

const antIcon = <LoadingOutlined style={{ fontSize: 24 }} spin />;

const Mine = () => {
    const [messageApi, contextHolder] = message.useMessage();

    useEffect(() => {
    }, []);

    return (<>
            {contextHolder}
            <Card bordered title={"Mine for Tokens"} style={{ width:"50%", margin: "auto" }}>
              <Text italic>To get TKN, you must complete a PoW.</Text>
              <Spin indicator={antIcon} tip="Loading"/>
              <Button type="primary" style={{ width:"100%" }}>Start</Button>
            </Card>
    </>);
}
export default Mine;
