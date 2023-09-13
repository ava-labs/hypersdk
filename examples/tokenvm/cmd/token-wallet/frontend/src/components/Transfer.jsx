import React, {useEffect, useState, useRef } from "react";
import { Layout,  Divider, Space, App, Card, Form, Input, InputNumber, Button, Select } from "antd";
import { PlusOutlined } from '@ant-design/icons';
import { GetBalance, Transfer as Send } from "../../wailsjs/go/main/App";
const { Sider, Content } = Layout;

let index = 0;

const Transfer = () => {
    const { message } = App.useApp();
    const [balance, setBalance] = useState([]);
    const [transferForm] = Form.useForm();
    const key = "updatable";

    const getBalance = async () => {
        const bals = await GetBalance();
        console.log(bals);
        const parsedBalances = [];
        for(let i=0; i<bals.length; i++){
          parsedBalances.push({value: bals[i].ID, label:bals[i].Bal});
        }
        setBalance(parsedBalances);
    };

    const [items, setItems] = useState([]);
    const [name, setName] = useState('');
    const onNameChange = (event) => {
      setName(event.target.value);
    };

    const addItem = (e) => {
      e.preventDefault();
      setItems([...items, name || `New item ${index++}`]);
      setName('');
      {/* close: https://codesandbox.io/s/ji-ben-shi-yong-antd-4-21-7-forked-gnp4cy?file=/demo.js */}
    };


    const onFinishTransfer = (values) => {
      console.log('Success:', values);
      transferForm.resetFields();

      message.open({key, type: "loading", content: "Issuing Transaction...", duration:0});
      (async () => {
        try {
          const start = (new Date()).getTime();
          await Send(values.Asset, values.Address, values.Amount);
          const finish = (new Date()).getTime();
          message.open({
            key, type: "success", content: `Transaction Finalized (${finish-start} ms)`,
          });
          getBalance();
        } catch (e) {
          message.open({
            key, type: "error", content: e.toString(),
          });
        }
      })();
    };
    
    const onFinishTransferFailed = (errorInfo) => {
      console.log('Failed:', errorInfo);
    };

    useEffect(() => {
        getBalance();
    }, []);

    return (<>
            <Card bordered title={"Send a Token"} style={{ width:"50%", margin: "auto" }}>
              <Form
                name="basic"
                form={transferForm}
                initialValues={{ remember: false }}
                onFinish={onFinishTransfer}
                onFinishFailed={onFinishTransferFailed}
                autoComplete="off"
              >
                <Form.Item name="Address" rules={[{ required: true }]}>
      {/* <Input placeholder="Address" /> */}
                  <Select
                    placeholder="Address"
                    dropdownRender={(menu) => (
                      <>
                        {menu}
                        <Divider style={{ margin: '8px 0' }} />
                        <Layout hasSider>
                          <Content style={{ backgroundColor: "white" }}>
                          <Input
                            placeholder="Nickname"
                            value={name}
                            onChange={onNameChange}
                            style={{ margin: '0 0 8px 0' }}
                          />
                          <br />
                          <Input
                            placeholder="Address"
                            value={name}
                            onChange={onNameChange}
                          />
                          </Content>
                          <Sider width="30%">
                          <div style={{ height:"100%", "display":"flex", "justify-content":"center", "align-items":"center", "backgroundColor": "white"}}>
                          <Button type="text" icon={<PlusOutlined />} onClick={addItem} >
                            Add item
                          </Button>
                          </div>
                          </Sider>
                        </Layout>
                      </>
                    )}
                    options={items.map((item) => ({ label: item, value: item }))}
                  />
                </Form.Item>
                <Form.Item name="Asset" rules={[{ required: true }]}>
                  <Select placeholder="Token" options={balance} />
                </Form.Item>
                <Form.Item name="Amount" rules={[{ required: true }]}>
                  <InputNumber placeholder="Amount" min={0} stringMode="true" style={{ width:"100%" }}/>
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit">
                    Send
                  </Button>
                </Form.Item>
              </Form>
            </Card>
    </>);
}
export default Transfer;
