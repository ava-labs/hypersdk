import {useEffect, useState} from "react";
import { Card, Form, Input, InputNumber, Button, Select, message } from "antd";
import { GetBalance, Transfer as Send } from "../../wailsjs/go/main/App";

const Transfer = () => {
    const [balance, setBalance] = useState([]);
    const [messageApi, contextHolder] = message.useMessage();
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


    const onFinishTransfer = (values) => {
      console.log('Success:', values);
      transferForm.resetFields();

      messageApi.open({key, type: "loading", content: "Issuing Transaction...", duration:0});
      (async () => {
        try {
          const start = (new Date()).getTime();
          await Send(values.Asset, values.Address, values.Amount);
          const finish = (new Date()).getTime();
          messageApi.open({
            key, type: "success", content: `Transaction Finalized (${finish-start} ms)`,
          });
          getBalance();
        } catch (e) {
          messageApi.open({
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
            {contextHolder}
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
                  <Input placeholder="Address" />
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
