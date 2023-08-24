import {useState} from "react";
import {Affix, Card, Col, Row, Typography} from "antd";
import ListItem from "./ListItem";

const MasterDetail = ({title, items, getItemDescription, detailLayout}) => {
    const [selectedItem, setSelectedItem] = useState(null);

    return (<>
            <Row justify="center">
                <Col>
                    <Typography.Title level={3}>{title}</Typography.Title>
                </Col>
            </Row>
            <Row>
                <Col span={6}>
                    <Affix offsetTop={20}>
                        <div
                            id="scrollableDiv"
                            style={{
                                height: "80vh", overflow: "auto", padding: "0 5px",
                            }}
                        >
                            <Card bordered={false} style={{boxShadow: "none"}}>
                                {items.map((item, index) => (<ListItem
                                        key={index}
                                        item={item}
                                        onSelect={setSelectedItem}
                                        selectedItem={selectedItem}
                                        title={getItemDescription(item)}
                                    />))}
                            </Card>
                        </div>
                    </Affix>
                </Col>
                <Col span={18}>{selectedItem && detailLayout(selectedItem)}</Col>
            </Row>
        </>);
};

export default MasterDetail;
