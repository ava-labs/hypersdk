import { useEffect, useState, useRef } from "react";
import {
  Space,
  App,
  Drawer,
  Divider,
  List,
  Typography,
  Form,
  Input,
  InputNumber,
  Button,
  Select,
  Popover,
} from "antd";
import {
  PlusOutlined,
  DoubleRightOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons";
import {
  GetBalance,
  GetAllAssets,
  AddAsset,
  GetMyOrders,
  GetOrders,
  CreateOrder,
  CloseOrder,
} from "../../wailsjs/go/main/App";
import FundsCheck from "./FundsCheck";
const { Text } = Typography;
import FillOrder from "./FillOrder";

const Trade = () => {
  const { message } = App.useApp();
  const key = "updatable";

  {
    /* Create Handlers */
  }
  const [openCreate, setOpenCreate] = useState(false);
  const [createForm] = Form.useForm();
  const showCreateDrawer = () => {
    setOpenCreate(true);
  };

  const onCloseCreate = () => {
    createForm.resetFields();
    setOpenCreate(false);
  };

  const onFinishCreate = (values) => {
    console.log("Success:", values);
    createForm.resetFields();
    setOpenCreate(false);

    message.open({
      key,
      type: "loading",
      content: "Processing Transaction...",
      duration: 0,
    });
    (async () => {
      try {
        const start = new Date().getTime();
        await CreateOrder(
          values.InSymbol,
          values.InTick,
          values.OutSymbol,
          values.OutTick,
          values.Supply
        );
        const finish = new Date().getTime();
        message.open({
          key,
          type: "success",
          content: `Transaction Finalized (${finish - start} ms)`,
        });
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  const onFinishCreateFailed = (errorInfo) => {
    console.log("Failed:", errorInfo);
  };

  const [balance, setBalance] = useState([]);
  const getBalance = async () => {
    const bals = await GetBalance();
    console.log(bals);
    const parsedBalances = [];
    for (let i = 0; i < bals.length; i++) {
      parsedBalances.push({ value: bals[i].ID, label: bals[i].Bal });
    }
    setBalance(parsedBalances);
  };

  {
    /* Symbol Add (only on out) */
  }
  const [assets, setAssets] = useState([]);
  const [outAssets, setOutAssets] = useState([]);
  const [newAsset, setNewAsset] = useState("");
  const [addAllowed, setAddAllowed] = useState(false);
  const [outAllowed, setOutAllowed] = useState(false);

  {
    /* Must use references because we want latest value after construction */
  }
  const inAsset = useRef("");
  const outAsset = useRef("");

  const inSelected = (event) => {
    inAsset.current = event;
    if (event.length > 0) {
      outAsset.current = "";
      const limitedAssets = [];
      for (var asset of assets) {
        if (asset.ID == event) {
          continue;
        }
        limitedAssets.push(asset);
      }
      setOutAssets(limitedAssets);
      setOutAllowed(true);
    } else {
      setOutAllowed(false);
      outAsset.current = "";
      setOrders([]);
    }
  };
  const outSelected = (event) => {
    outAsset.current = event;
    getOrders();
  };

  const onAssetChange = (event) => {
    setNewAsset(event.target.value);
    if (event.target.value.length > 0) {
      setAddAllowed(true);
    } else {
      setAddAllowed(false);
    }
  };

  const getAllAssets = async () => {
    const allAssets = await GetAllAssets();
    setAssets(allAssets);
  };

  const addAsset = (e) => {
    e.preventDefault();
    (async () => {
      try {
        await AddAsset(newAsset);
        setNewAsset("");
        const allAssets = await GetAllAssets();
        setAssets(allAssets);
        const limitedAssets = [];
        for (var asset of allAssets) {
          if (asset.ID == inAsset.current) {
            continue;
          }
          limitedAssets.push(asset);
        }
        setOutAssets(limitedAssets);
        message.open({
          type: "success",
          content: `${newAsset} added`,
        });
      } catch (e) {
        message.open({
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  const [myOrders, setMyOrders] = useState([]);
  const getMyOrders = async () => {
    const newMyOrders = await GetMyOrders();
    setMyOrders(newMyOrders);
  };

  const [orders, setOrders] = useState([]);
  const getOrders = async () => {
    if (inAsset.current.length == 0 || outAsset.current.length == 0) {
      console.log(inAsset.current, outAsset.current);
      setOrders([]);
      return;
    }
    const newOrders = await GetOrders(`${inAsset.current}-${outAsset.current}`);
    console.log(newOrders);
    setOrders(newOrders);
  };

  const closeOrder = (values) => {
    console.log("Success:", values);

    message.open({
      key,
      type: "loading",
      content: "Processing Transaction...",
      duration: 0,
    });
    (async () => {
      try {
        const start = new Date().getTime();
        await CloseOrder(values.ID, values.OutID);
        const finish = new Date().getTime();
        await getMyOrders();
        message.open({
          key,
          type: "success",
          content: `Transaction Finalized (${finish - start} ms)`,
        });
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  useEffect(() => {
    getMyOrders();
    getOrders();
    getBalance();
    getAllAssets();
    const interval = setInterval(() => {
      getMyOrders();
      getOrders();
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return (
    <>
      <div style={{ width: "60%", margin: "auto" }}>
        <FundsCheck />
        <Divider orientation="center">
          Open Orders
          <Popover
            content={
              <div>
                On TokenNet, anyone can trade the tokens they own.
                <br />
                <br />
                Once you create an order (i.e. offer to buy/sell some token), it
                will show up below!
                <br />
                <br />
                When closing an order, all unsold funds will return to you.
              </div>
            }>
            {" "}
            <InfoCircleOutlined />
          </Popover>
        </Divider>
        <div style={{ display: "flex", width: "100%" }}>
          <Button
            type="primary"
            onClick={showCreateDrawer}
            placement={"right"}
            style={{ margin: "0 0 8px 0", "margin-left": "auto" }}
            disabled={!window.HasBalance}>
            Create Order
          </Button>
        </div>
        <List
          bordered
          dataSource={myOrders}
          renderItem={(item) => (
            <List.Item>
              <Text strong style={{ color: "red" }}>
                {item.InTick}
              </Text>{" "}
              /{" "}
              <Text strong style={{ color: "green" }}>
                {item.OutTick}
              </Text>{" "}
              <Text type="secondary">[Rate: {item.Rate}]</Text>
              <br />
              <Text strong>Remaining:</Text> {item.Remaining} (Max Trade:{" "}
              {item.MaxInput})
              <br />
              <Text strong>ID:</Text> {item.ID}
              <br />
              <Button
                type="primary"
                danger
                onClick={() => closeOrder(item)}
                disabled={!window.HasBalance}
                style={{ margin: "8px 0 0 0" }}>
                Close
              </Button>
            </List.Item>
          )}
        />
        <Divider orientation="center">
          Order Book
          <Popover
            content={
              <div>
                The "Order Book" contains all orders for a given pair of tokens
                sorted by rate (best to worst).
                <br />
                <br />
                You can fill an order for any pair where you hold the "in"
                token.
                <br />
                <br />
                Orders on TokenNet are executed as a batch. For each "tick" of
                the input amount, you receive a "tick" of the output amount.
                <br />
                <br />
                <Text italic>
                  Example: If you created an order of "2 TKN / 5 YOU" (where
                  "YOU" is your token), anyone could pay "2 TKN" to get "5 YOU".
                </Text>
              </div>
            }>
            {" "}
            <InfoCircleOutlined />
          </Popover>
        </Divider>
        <div
          style={{
            justifyContent: "space-between",
            alignItems: "center",
            display: "flex",
            margin: "0 0 8px 0",
          }}>
          <Select
            placeholder="In"
            style={{ width: "45%" }}
            options={balance}
            onChange={inSelected}
          />
          <DoubleRightOutlined style={{ fontSize: "15px" }} />
          <Select
            placeholder="Out"
            style={{ width: "45%" }}
            disabled={!outAllowed}
            value={outAsset.current}
            onChange={outSelected}
            dropdownRender={(menu) => (
              <>
                {menu}
                <Divider style={{ margin: "8px 0" }} />
                <Space style={{ padding: "0 8px 4px" }}>
                  <Input
                    placeholder="Asset"
                    value={newAsset}
                    onChange={onAssetChange}
                    allowClear
                  />
                  <Button
                    type="text"
                    icon={<PlusOutlined />}
                    onClick={addAsset}
                    disabled={!addAllowed}></Button>
                </Space>
              </>
            )}
            options={outAssets.map((item) => ({
              label: item.StrSymbol,
              value: item.ID,
            }))}
          />
        </div>
        <List
          bordered
          dataSource={orders}
          renderItem={(item) => (
            <List.Item>
              <Text strong style={{ color: "red" }}>
                {item.InTick}
              </Text>{" "}
              /{" "}
              <Text strong style={{ color: "green" }}>
                {item.OutTick}
              </Text>{" "}
              <Text type="secondary">[Rate: {item.Rate}]</Text>
              <br />
              <Text strong>Remaining:</Text> {item.Remaining} (Max Trade:{" "}
              {item.MaxInput})
              <br />
              <Text strong>ID:</Text> {item.ID}
              <br />
              <Text strong>Owner:</Text> {item.Owner}
              <FillOrder order={item} />
            </List.Item>
          )}
        />
      </div>
      <Drawer
        title={
          <>
            Create Order
            <Popover
              content={
                <div>
                  Orders on TokenNet are executed as a batch. For each "tick" of
                  the input amount, you receive a "tick" of the output amount.
                  <br />
                  <br />
                  When creating an order, the "supply" of tokens you will trade
                  for the "token you buy" is locked.
                  <br />
                  <br />
                  When the order is filled, you will receive the "token you buy"
                  and the counterparty will receive "the token you sell" (what
                  you have).
                  <br />
                  <br />
                  <Text italic>
                    Example: If you created an order of "2 TKN / 5 YOU" (where
                    "YOU" is your token), anyone could pay "2 TKN" to get "5
                    YOU".
                  </Text>
                </div>
              }>
              {" "}
              <InfoCircleOutlined />
            </Popover>
          </>
        }
        placement={"right"}
        onClose={onCloseCreate}
        open={openCreate}>
        <Form
          name="basic"
          form={createForm}
          initialValues={{ remember: false }}
          onFinish={onFinishCreate}
          onFinishFailed={onFinishCreateFailed}
          autoComplete="off">
          {/* inSymbol, inTick, outSymbol, outTick, supply (multiple of out tick) */}
          <Form.Item
            name="InSymbol"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <Select
              placeholder="Token You Buy"
              dropdownRender={(menu) => (
                <>
                  {menu}
                  <Divider style={{ margin: "8px 0" }} />
                  <Space style={{ padding: "0 8px 4px" }}>
                    <Input
                      placeholder="Asset"
                      value={newAsset}
                      onChange={onAssetChange}
                      allowClear
                    />
                    <Button
                      type="text"
                      icon={<PlusOutlined />}
                      onClick={addAsset}
                      disabled={!addAllowed}></Button>
                  </Space>
                </>
              )}
              options={assets.map((item) => ({
                label: item.StrSymbol,
                value: item.ID,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="InTick"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <InputNumber
              placeholder="Batch Amount You Buy"
              stringMode="true"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item
            name="OutSymbol"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <Select placeholder="Token You Sell" options={balance} />
          </Form.Item>
          <Form.Item
            name="OutTick"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <InputNumber
              placeholder="Batch Amount You Sell"
              stringMode="true"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item
            name="Supply"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <InputNumber
              placeholder="Order Size of Token You Sell"
              stringMode="true"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              style={{ margin: "0 0 8px 0" }}>
              Create
            </Button>
          </Form.Item>
        </Form>
      </Drawer>
    </>
  );
};
export default Trade;
