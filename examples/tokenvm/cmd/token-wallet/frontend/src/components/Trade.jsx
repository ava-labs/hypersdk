import {useEffect, useState, useRef} from "react";
import { Space, FloatButton, App, Drawer, Divider, List, Card, Typography, Form, Input, InputNumber, Button, Select, Spin } from "antd";
import { CheckCircleTwoTone, CloseCircleTwoTone, LoadingOutlined, PlusOutlined, DoubleRightOutlined } from '@ant-design/icons';
import { GetBalance, GetAllAssets, AddAsset, GetOrders } from "../../wailsjs/go/main/App";
const { Text, Title, Link } = Typography;
import FillOrder from "./FillOrder";

const Trade = () => {
    const { message } = App.useApp();
    const key = "updatable";

    {/* Create Handlers */}
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
      console.log('Success:', values);
      createForm.resetFields();
      setOpenCreate(false);

      message.open({key, type: "loading", content: "Processing Transaction...", duration:0});
      (async () => {
        try {
          const start = (new Date()).getTime();
          await CreateAsset(values.Symbol, values.Decimals, values.Metadata);
          const finish = (new Date()).getTime();
          message.open({
            key, type: "success", content: `Transaction Finalized (${finish-start} ms)`,
          });
        } catch (e) {
          message.open({
            key, type: "error", content: e.toString(),
          });
        }
      })();
    };
    
    const onFinishCreateFailed = (errorInfo) => {
      console.log('Failed:', errorInfo);
    };

    const [balance, setBalance] = useState([]);
    const getBalance = async () => {
        const bals = await GetBalance();
        console.log(bals);
        const parsedBalances = [];
        for(let i=0; i<bals.length; i++){
          parsedBalances.push({value: bals[i].ID, label:bals[i].Bal});
        }
        setBalance(parsedBalances);
    };

    {/* Symbol Add (only on out) */}
    const [assets, setAssets] = useState([]);
    const [outAssets, setOutAssets] = useState([]);
    const [newAsset, setNewAsset] = useState('');
    const [addAllowed, setAddAllowed] = useState(false);
    const [outAllowed, setOutAllowed] = useState(false);

    {/* Must use references because we want latest value after construction */}
    const inAsset = useRef('');
    const outAsset = useRef('');
    
    const inSelected = (event) => {
      inAsset.current = event;
      if (event.length > 0) {
        outAsset.current = '';
        const limitedAssets = [];
        for (var asset of assets) {
          if (asset.ID == event) {
            continue
          }
          limitedAssets.push(asset)
        }
        setOutAssets(limitedAssets);
        setOutAllowed(true);
      } else {
        setOutAllowed(false);
        outAsset.current = '';
        setOrders([]);
      }
    }
    const outSelected = (event) => {
      outAsset.current = event;
      getOrders();
    }
    
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
          setNewAsset('');
          const allAssets = await GetAllAssets();
          setAssets(allAssets);
          const limitedAssets = [];
          for (var asset of allAssets) {
            if (asset.ID == inAsset.current) {
              continue
            }
            limitedAssets.push(asset)
          }
          setOutAssets(limitedAssets);
          message.open({
            type: "success", content: `${newAsset} added`,
          });
        } catch (e) {
          message.open({
            type: "error", content: e.toString(),
          });
        }
      })();
    };

    const [orders, setOrders] = useState([]);
    const getOrders = async () => {
        if (inAsset.current.length == 0 || outAsset.current.length == 0) {
          console.log(inAsset.current, outAsset.current);
          setOrders([]);
          return
        }
        const newOrders = await GetOrders(`${inAsset.current}-${outAsset.current}`);
        console.log(newOrders);
        setOrders(newOrders);
    };

    useEffect(() => {
      getOrders();
      getBalance();
      getAllAssets();
      const interval = setInterval(() => {
        getOrders();
      }, 500);

      return () => clearInterval(interval);
    }, []);

    return (<>
      <FloatButton icon={<PlusOutlined />} type="primary" onClick={showCreateDrawer} />
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
      <div style={{ "justifyContent": "space-between", "alignItems": "center", "display": "flex", "margin":"0 0 8px 0" }} >
      <Select placeholder="In" style={{ width:"45%" }} options={balance} onChange={inSelected}/>
      <DoubleRightOutlined style={{ fontSize: "15px" }}/>
      <Select placeholder="Out" style={{ width:"45%", }} disabled={!outAllowed} value={outAsset.current} onChange={outSelected}
        dropdownRender={(menu) => (
          <>
            {menu}
            <Divider style={{ margin: '8px 0' }} />
            <Space style={{ padding: '0 8px 4px' }}>
              <Input
                placeholder="Asset"
                value={newAsset}
                onChange={onAssetChange}
                allowClear
              />
              <Button type="text" icon={<PlusOutlined />} onClick={addAsset} disabled={!addAllowed}></Button>
            </Space>
          </>
        )}
        options={outAssets.map((item) => ({ label: item.StrSymbol, value: item.ID}))}
      />
      </div>
      <List
        bordered
        dataSource={orders}
        renderItem={(item) => (
          <List.Item>
            <Text strong style={{ color: "red" }}>{item.InTick}</Text> / <Text strong style={{ color:"green" }}>{item.OutTick}</Text>
            <br />
            <Text strong>Remaining:</Text> {item.Remaining} (Max Trade: {item.MaxInput})
            <br />
            <Text strong>ID:</Text> {item.ID}
            <br />
            <Text strong>Owner:</Text> {item.Owner}
            <FillOrder order={item}/>
          </List.Item>
        )}
      />
      <Divider orientation="center">
        Explanation
      </Divider>
      </div>
      <Drawer title={"Create an Order"} placement={"right"} onClose={onCloseCreate} open={openCreate}>
        <Form
          name="basic"
          form={createForm}
          initialValues={{ remember: false }}
          onFinish={onFinishCreate}
          onFinishFailed={onFinishCreateFailed}
          autoComplete="off"
        >
          {/* inSymbol, inTick, outSymbol, outTick, supply (multiple of out tick) */}
          <Form.Item name="InSymbol" rules={[{ required: true }]}>
            <Select placeholder="Token You Buy"
              dropdownRender={(menu) => (
                <>
                  {menu}
                  <Divider style={{ margin: '8px 0' }} />
                  <Space style={{ padding: '0 8px 4px' }}>
                    <Input
                      placeholder="Asset"
                      value={newAsset}
                      onChange={onAssetChange}
                      allowClear
                    />
                    <Button type="text" icon={<PlusOutlined />} onClick={addAsset} disabled={!addAllowed}></Button>
                  </Space>
                </>
              )}
              options={assets.map((item) => ({ label: item.StrSymbol, value: item.ID}))}
            />
          </Form.Item>
          <Form.Item name="InTick" rules={[{ required: true }]}>
            <InputNumber placeholder="Batch Amount You Buy" stringMode="true" style={{ width:"100%" }}/>
          </Form.Item>
          <Form.Item name="OutSymbol" rules={[{ required: true }]}>
            <Select placeholder="Token You Sell" options={balance} />
          </Form.Item>
          <Form.Item name="OutTick" rules={[{ required: true }]}>
            <InputNumber placeholder="Batch Amount You Sell" stringMode="true" style={{ width:"100%" }}/>
          </Form.Item>
          <Form.Item name="Supply" rules={[{ required: true }]}>
            <InputNumber placeholder="Order Size of Token You Sell" stringMode="true" style={{ width:"100%" }}/>
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit">
              Create
            </Button>
          </Form.Item>
        </Form>
      <Divider orientation="center">
        Explanation
      </Divider>
        <Text italic>Trades</Text>
      </Drawer>
    </>);
}
export default Trade;
