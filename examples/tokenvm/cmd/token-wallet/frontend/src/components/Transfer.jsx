import React, { useEffect, useState } from "react";
import {
  Divider,
  Space,
  App,
  Card,
  Form,
  Input,
  InputNumber,
  Button,
  Select,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  GetBalance,
  Transfer as Send,
  AddAddressBook,
  GetAddressBook,
} from "../../wailsjs/go/main/App";
import FundsCheck from "./FundsCheck";

const Transfer = () => {
  const { message } = App.useApp();
  const [balance, setBalance] = useState([]);
  const [transferForm] = Form.useForm();
  const key = "updatable";

  const getBalance = async () => {
    const bals = await GetBalance();
    console.log(bals);
    const parsedBalances = [];
    for (let i = 0; i < bals.length; i++) {
      parsedBalances.push({ value: bals[i].ID, label: bals[i].Bal });
    }
    setBalance(parsedBalances);
  };

  const [addresses, setAddresses] = useState([]);
  const [newNickname, setNewNickname] = useState("");
  const [newAddress, setNewAddress] = useState("");
  const [addAllowed, setAddAllowed] = useState(false);

  const onNicknameChange = (event) => {
    setNewNickname(event.target.value);
    if (event.target.value.length > 0 && newAddress.length > 10) {
      setAddAllowed(true);
    } else {
      setAddAllowed(false);
    }
  };
  const onAddressChange = (event) => {
    setNewAddress(event.target.value);
    if (newNickname.length > 0 && event.target.value.length > 10) {
      setAddAllowed(true);
    } else {
      setAddAllowed(false);
    }
  };

  const getAddresses = async () => {
    const caddresses = await GetAddressBook();
    setAddresses(caddresses);
  };

  const addAddress = (e) => {
    e.preventDefault();
    (async () => {
      try {
        await AddAddressBook(newNickname, newAddress);
        setNewNickname("");
        setNewAddress("");
        await getAddresses();
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  const onFinishTransfer = (values) => {
    console.log("Success:", values);
    transferForm.resetFields();

    message.open({
      key,
      type: "loading",
      content: "Processing Transaction...",
      duration: 0,
    });
    (async () => {
      try {
        const start = new Date().getTime();
        await Send(values.Asset, values.Address, values.Amount, values.Memo);
        const finish = new Date().getTime();
        message.open({
          key,
          type: "success",
          content: `Transaction Finalized (${finish - start} ms)`,
        });
        getBalance();
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  const onFinishTransferFailed = (errorInfo) => {
    console.log("Failed:", errorInfo);
  };

  useEffect(() => {
    getBalance();
    getAddresses();
  }, []);

  return (
    <>
      <div style={{ width: "60%", margin: "auto" }}>
        <FundsCheck />
        <Card bordered title={"Send a Token"}>
          <Form
            name="basic"
            form={transferForm}
            initialValues={{ remember: false }}
            onFinish={onFinishTransfer}
            onFinishFailed={onFinishTransferFailed}
            autoComplete="off">
            <Form.Item
              name="Address"
              rules={[{ required: true }]}
              style={{ margin: "0 0 8px 0" }}>
              <Select
                placeholder="Recipient"
                dropdownRender={(menu) => (
                  <>
                    {menu}
                    <Divider style={{ margin: "8px 0" }} />
                    <Space style={{ padding: "0 8px 4px" }}>
                      <Input
                        placeholder="Nickname"
                        value={newNickname}
                        onChange={onNicknameChange}
                        allowClear
                      />
                      <Input
                        placeholder="Address"
                        value={newAddress}
                        onChange={onAddressChange}
                        allowClear
                      />
                      <Button
                        type="text"
                        icon={<PlusOutlined />}
                        onClick={addAddress}
                        disabled={!addAllowed}></Button>
                    </Space>
                  </>
                )}
                options={addresses.map((item) => ({
                  label: item.AddrStr,
                  value: item.Address,
                }))}
              />
            </Form.Item>
            <Form.Item
              name="Asset"
              rules={[{ required: true }]}
              style={{ margin: "0 0 8px 0" }}>
              <Select placeholder="Token" options={balance} />
            </Form.Item>
            <Form.Item
              name="Amount"
              rules={[{ required: true }]}
              style={{ margin: "0 0 8px 0" }}>
              <InputNumber
                placeholder="Amount"
                min={0}
                stringMode="true"
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item
              name="Memo"
              rules={[{ required: false }]}
              style={{ margin: "0 0 8px 0" }}>
              <Input placeholder="Memo" maxLength="256" />
            </Form.Item>
            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                style={{ margin: "0 0 8px 0" }}
                disabled={!window.HasBalance}>
                Send
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </div>
    </>
  );
};
export default Transfer;
