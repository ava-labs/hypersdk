import { useEffect, useState } from "react";
import { Avatar, Card, Skeleton } from "antd";

const ListItem = ({ item, onSelect, selectedItem, title }) => {
  const [loading, setLoading] = useState(true);
  const [gridStyle, setGridStyle] = useState({
    margin: "3%",
    width: "94%",
  });

  useEffect(() => {
    const isSelected = selectedItem?.id === item.id;
    setGridStyle({
        margin: "3%",
        width: "94%",
      ...(isSelected && { backgroundColor: "lightblue" }),
    });

  }, [selectedItem]);

  const onClickHandler = () => {
    onSelect(item);
  };

  useEffect(() => {
    setTimeout(() => {
      setLoading(false);
    }, 3000);
  }, []);

  return (
    <Card.Grid hoverable={true} style={gridStyle} onClick={onClickHandler}>
      <Skeleton loading={loading} avatar active>
        <Card.Meta
          avatar={<Avatar src="https://cdn.pixabay.com/photo/2021/06/17/22/55/rick-and-morty-6344804_1280.jpg" />}
          title={title}
          description={`Authored by `}
        />
      </Skeleton>
    </Card.Grid>
  );
};

export default ListItem;
